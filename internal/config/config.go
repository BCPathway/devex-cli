// Package config handles loading, merging, and validating the devex CLI
// configuration from .devex.yaml files and environment variables.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level application configuration.
type Config struct {
	Project ProjectConfig `yaml:"project" json:"project" mapstructure:"project"`
	Drips   DripsConfig   `yaml:"drips"   json:"drips"   mapstructure:"drips"`
	Stellar StellarConfig `yaml:"stellar" json:"stellar" mapstructure:"stellar"`
	Dev     DevConfig     `yaml:"dev"     json:"dev"     mapstructure:"dev"`
}

// ProjectConfig holds project-level metadata.
type ProjectConfig struct {
	Name          string `yaml:"name"           json:"name"           mapstructure:"name"`
	RepositoryURL string `yaml:"repository_url" json:"repository_url" mapstructure:"repository_url"`
}

// DripsConfig holds Drips Network connection settings.
type DripsConfig struct {
	RPCEndpoint   string `yaml:"rpc_endpoint"   json:"rpc_endpoint"   mapstructure:"rpc_endpoint"`
	ChainID       int    `yaml:"chain_id"       json:"chain_id"       mapstructure:"chain_id"`
	WalletAddress string `yaml:"wallet_address" json:"wallet_address" mapstructure:"wallet_address"`
	PrivateKey    string `yaml:"private_key"    json:"-"              mapstructure:"private_key"` // never serialise
}

// StellarConfig holds Stellar Network connection settings.
type StellarConfig struct {
	HorizonURL        string `yaml:"horizon_url"        json:"horizon_url"        mapstructure:"horizon_url"`
	NetworkPassphrase string `yaml:"network_passphrase" json:"network_passphrase" mapstructure:"network_passphrase"`
	AccountID         string `yaml:"account_id"         json:"account_id"         mapstructure:"account_id"`
	SecretKey         string `yaml:"secret_key"         json:"-"                  mapstructure:"secret_key"` // never serialise
}

// DevConfig holds local development environment settings.
type DevConfig struct {
	StartCommand string   `yaml:"start_command" json:"start_command" mapstructure:"start_command"`
	Services     []string `yaml:"services"      json:"services"      mapstructure:"services"`
}

// Load reads configuration from the specified file (or default locations)
// and merges in environment variables.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// --- File-based config ---
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName(".devex")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")

		// Also check home directory for a global config.
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(home)
		}
	}

	// --- Environment variable bindings ---
	// Prefix: DEVEX_  e.g. DEVEX_DRIPS_RPC_ENDPOINT
	v.SetEnvPrefix("DEVEX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit legacy / shorthand env vars (no prefix).
	bindEnvVars(v)

	// --- Read file (non-fatal if missing) ---
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		// File not found is acceptable — env-only mode.
	}

	// --- Unmarshal ---
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// Default returns a Config with sensible zero-value defaults.
func Default() *Config {
	return &Config{
		Drips: DripsConfig{
			RPCEndpoint: "https://mainnet.optimism.io",
			ChainID:     10,
		},
		Stellar: StellarConfig{
			HorizonURL:        "https://horizon-testnet.stellar.org",
			NetworkPassphrase: "Test SDF Network ; September 2015",
		},
		Dev: DevConfig{
			StartCommand: "docker compose up",
		},
	}
}

// bindEnvVars maps well-known environment variable names to config keys.
// These supplement the automatic DEVEX_ prefix binding.
func bindEnvVars(v *viper.Viper) {
	envMap := map[string]string{
		"drips.rpc_endpoint":         "DRIPS_NETWORK_RPC",
		"drips.private_key":          "DRIPS_PRIVATE_KEY",
		"drips.wallet_address":       "DRIPS_WALLET_ADDRESS",
		"drips.chain_id":             "DRIPS_CHAIN_ID",
		"stellar.horizon_url":        "STELLAR_HORIZON_URL",
		"stellar.network_passphrase": "STELLAR_NETWORK_PASSPHRASE",
		"stellar.account_id":         "STELLAR_ACCOUNT_ID",
		"stellar.secret_key":         "STELLAR_SECRET_KEY",
	}
	for key, env := range envMap {
		_ = v.BindEnv(key, env)
	}
}
