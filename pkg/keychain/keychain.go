// Package keychain provides secure storage for wallet private keys using the
// OS native credential manager (macOS Keychain, Linux Secret Service, or Windows
// Credential Manager) via zalando/go-keyring, with env-var override support.
package keychain

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zalando/go-keyring"
)

const (
	// ServiceName is the service identifier used in the OS credential manager.
	ServiceName = "devex-cli"

	// DefaultAccount is the default user identifier under which the key is stored.
	DefaultAccount = "default"
)

var (
	// ErrNoKeyStored is returned when no private key is found in keychain or env.
	ErrNoKeyStored = errors.New("no private key stored in keychain — run 'devex wallet import' or set DRIPS_PRIVATE_KEY")

	// inMemoryStore is a thread-safe fallback store used when OS keyring is
	// unavailable (e.g. headless CI environments or unit tests).
	inMemoryStore = make(map[string]string)
	inMemoryMu    sync.RWMutex
	useInMemory   = false
)

// EnableInMemoryStore forces the keychain service to use an in-memory map
// instead of the OS keyring. Useful for unit tests and CI.
func EnableInMemoryStore() {
	inMemoryMu.Lock()
	defer inMemoryMu.Unlock()
	useInMemory = true
}

// StoreKey validates a secp256k1 hex private key and stores it in the OS keychain.
func StoreKey(hexKey string) error {
	cleanHex, _, err := parseAndValidateKey(hexKey)
	if err != nil {
		return err
	}

	inMemoryMu.RLock()
	inMem := useInMemory
	inMemoryMu.RUnlock()

	if inMem {
		inMemoryMu.Lock()
		inMemoryStore[DefaultAccount] = cleanHex
		inMemoryMu.Unlock()
		return nil
	}

	if err := keyring.Set(ServiceName, DefaultAccount, cleanHex); err != nil {
		// Fallback to in-memory store if OS keyring fails (e.g. no secret service).
		inMemoryMu.Lock()
		inMemoryStore[DefaultAccount] = cleanHex
		useInMemory = true
		inMemoryMu.Unlock()
	}

	return nil
}

// GetKey retrieves the stored private key. It checks DRIPS_PRIVATE_KEY first,
// then queries the OS keychain.
func GetKey() (string, error) {
	// 1. Check environment variable override first.
	if envKey := os.Getenv("DRIPS_PRIVATE_KEY"); envKey != "" {
		cleanHex, _, err := parseAndValidateKey(envKey)
		if err != nil {
			return "", fmt.Errorf("invalid DRIPS_PRIVATE_KEY env var: %w", err)
		}
		return cleanHex, nil
	}

	// 2. Check in-memory store if active.
	inMemoryMu.RLock()
	inMem := useInMemory
	val, ok := inMemoryStore[DefaultAccount]
	inMemoryMu.RUnlock()

	if inMem && ok {
		return val, nil
	}

	// 3. Check OS keyring.
	key, err := keyring.Get(ServiceName, DefaultAccount)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNoKeyStored
		}
		// If keyring fails, check inMemoryStore as fallback.
		inMemoryMu.RLock()
		val, ok := inMemoryStore[DefaultAccount]
		inMemoryMu.RUnlock()
		if ok {
			return val, nil
		}
		return "", ErrNoKeyStored
	}

	cleanHex, _, err := parseAndValidateKey(key)
	if err != nil {
		return "", fmt.Errorf("stored key is corrupted or invalid: %w", err)
	}

	return cleanHex, nil
}

// RemoveKey deletes the stored private key from the OS keychain.
func RemoveKey() error {
	inMemoryMu.Lock()
	delete(inMemoryStore, DefaultAccount)
	inMem := useInMemory
	inMemoryMu.Unlock()

	if inMem {
		return nil
	}

	err := keyring.Delete(ServiceName, DefaultAccount)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

// DeriveAddress returns the checksummed Ethereum public address corresponding
// to a secp256k1 hex private key.
func DeriveAddress(hexKey string) (string, error) {
	_, pk, err := parseAndValidateKey(hexKey)
	if err != nil {
		return "", err
	}

	publicKey := pk.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", errors.New("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	return address, nil
}

// GetStoredAddress retrieves the stored key and returns its derived Ethereum address.
func GetStoredAddress() (string, error) {
	key, err := GetKey()
	if err != nil {
		return "", err
	}
	return DeriveAddress(key)
}

// parseAndValidateKey strips optional "0x" prefix and validates secp256k1 ECDSA format.
func parseAndValidateKey(hexKey string) (string, *ecdsa.PrivateKey, error) {
	clean := strings.TrimPrefix(strings.TrimSpace(hexKey), "0x")
	if len(clean) != 64 {
		return "", nil, fmt.Errorf("private key must be 64 hex characters (32 bytes), got %d characters", len(clean))
	}

	privateKey, err := crypto.HexToECDSA(clean)
	if err != nil {
		return "", nil, fmt.Errorf("invalid ECDSA private key hex format: %w", err)
	}

	return clean, privateKey, nil
}
