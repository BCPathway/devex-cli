package keychain_test

import (
	"os"
	"testing"

	"github.com/BCPathway/devex-cli/pkg/keychain"
)

// Sample valid secp256k1 private key (test account only — never use on mainnet)
const testHexKey = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
const testExpectedAddress = "0x2c7536E3605D9C16a7a3D7b1898e529396a65c23"

func TestKeychainStoreAndRetrieve(t *testing.T) {
	keychain.EnableInMemoryStore()
	defer keychain.RemoveKey()

	// 1. Initially empty
	_, err := keychain.GetKey()
	if err != keychain.ErrNoKeyStored {
		t.Fatalf("expected ErrNoKeyStored, got %v", err)
	}

	// 2. Store valid key (with 0x prefix)
	if err := keychain.StoreKey("0x" + testHexKey); err != nil {
		t.Fatalf("StoreKey failed: %v", err)
	}

	// 3. Retrieve key (should be stripped of 0x)
	got, err := keychain.GetKey()
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}
	if got != testHexKey {
		t.Errorf("GetKey = %s, want %s", got, testHexKey)
	}

	// 4. Check derived address
	addr, err := keychain.GetStoredAddress()
	if err != nil {
		t.Fatalf("GetStoredAddress failed: %v", err)
	}
	if addr != testExpectedAddress {
		t.Errorf("GetStoredAddress = %s, want %s", addr, testExpectedAddress)
	}

	// 5. Remove key
	if err := keychain.RemoveKey(); err != nil {
		t.Fatalf("RemoveKey failed: %v", err)
	}
	_, err = keychain.GetKey()
	if err != keychain.ErrNoKeyStored {
		t.Errorf("expected ErrNoKeyStored after removal, got %v", err)
	}
}

func TestKeychainInvalidKey(t *testing.T) {
	keychain.EnableInMemoryStore()
	defer keychain.RemoveKey()

	err := keychain.StoreKey("invalid-short-key")
	if err == nil {
		t.Fatal("expected error for short key, got nil")
	}
}

func TestKeychainEnvOverride(t *testing.T) {
	keychain.EnableInMemoryStore()
	defer keychain.RemoveKey()

	// Set env var override
	os.Setenv("DRIPS_PRIVATE_KEY", "0x"+testHexKey)
	defer os.Unsetenv("DRIPS_PRIVATE_KEY")

	got, err := keychain.GetKey()
	if err != nil {
		t.Fatalf("GetKey with env override failed: %v", err)
	}
	if got != testHexKey {
		t.Errorf("GetKey = %s, want %s", got, testHexKey)
	}
}
