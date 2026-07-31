# devex CLI

> Developer workflow automation with [Drips Network](https://drips.network) integration.

`devex` is a command-line tool that streamlines local developer workflows and integrates with the Drips Network for transparent, on-chain funding streams and dependency splits.

---

## Quick Start

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- (Optional) [Docker](https://docs.docker.com/get-docker/) for `devex dev`

### Build

```bash
# Clone and build
git clone https://github.com/BCPathway/devex-cli.git
cd devex-cli
go mod tidy
go build -o bin/devex .

# Or use make
make build
```

### Install globally

```bash
go install .
# or
make install
```

---

## Usage

### Initialise a project

```bash
devex init
```

Interactively creates a `.devex.yaml` configuration file in the current directory with Drips Network settings, project metadata, and dev-environment preferences.

### Start the dev environment

```bash
devex dev              # foreground
devex dev --detach     # background (adds -d for docker compose)
```

### Query funding status

```bash
devex funding status
devex funding status --account-id 0xYourAddress
devex funding status --json    # structured JSON output
```

### Configure split rules

```bash
# Preview a split configuration
devex funding split --add 0xUpstream1=25 --add 0xUpstream2=10

# Apply on-chain
devex funding split --add 0xUpstream1=25 --add 0xUpstream2=10 --apply

# View current splits
devex funding split
```

### Global flags

| Flag        | Short | Description                            |
|-------------|-------|----------------------------------------|
| `--verbose` | `-v`  | Enable debug output                    |
| `--config`  |       | Path to config file (default: `.devex.yaml`) |
| `--json`    |       | Output results as JSON                 |

---

## Configuration

`devex` reads configuration from three sources, merged in priority order:

1. **CLI flags** (highest priority)
2. **Environment variables**
3. **`.devex.yaml`** file (lowest priority)

### Environment Variables

| Variable              | Config Key             | Description                     |
|-----------------------|------------------------|---------------------------------|
| `DRIPS_NETWORK_RPC`   | `drips.rpc_endpoint`   | Ethereum JSON-RPC endpoint      |
| `DRIPS_PRIVATE_KEY`   | `drips.private_key`    | Wallet private key (writes)     |
| `DRIPS_WALLET_ADDRESS`| `drips.wallet_address` | Wallet address (reads)          |
| `DRIPS_CHAIN_ID`      | `drips.chain_id`       | Chain ID (10=OP Mainnet)        |

All config keys also work with a `DEVEX_` prefix (e.g., `DEVEX_DRIPS_RPC_ENDPOINT`).

### Sample config

Copy the example and customise:

```bash
cp .devex.yaml.example .devex.yaml
```

See [`.devex.yaml.example`](.devex.yaml.example) for all available fields.

---

## Project Structure

```
devex-cli/
├── main.go                     # Entrypoint
├── go.mod                      # Go module definition
├── Makefile                    # Build, test, lint targets
├── .devex.yaml.example         # Config template
├── .gitignore
├── README.md
│
├── cmd/                        # CLI command tree (Cobra)
│   ├── root.go                 # Root command, persistent flags, config bootstrap
│   ├── init.go                 # devex init
│   ├── dev.go                  # devex dev
│   ├── funding.go              # devex funding (parent)
│   ├── funding_status.go       # devex funding status
│   └── funding_split.go        # devex funding split
│
├── internal/                   # Private application packages
│   ├── config/
│   │   └── config.go           # Config loading (Viper + env vars)
│   └── logger/
│       └── logger.go           # Structured logging
│
└── pkg/                        # Public, reusable packages
    └── drips/
        ├── client.go           # Drips Network client (RPC connection)
        ├── types.go            # Domain types (StreamInfo, SplitEntry, etc.)
        ├── streams.go          # Stream query operations
        └── splits.go           # Split management + balance queries
```

---

## Extending the CLI

### Adding a new command

1. Create a new file in `cmd/` (e.g., `cmd/deploy.go`)
2. Define a `cobra.Command` and register it in `init()`:
   ```go
   func init() {
       rootCmd.AddCommand(deployCmd)
   }
   ```
3. For nested commands, add to a parent:
   ```go
   func init() {
       fundingCmd.AddCommand(myNewSubcmd)
   }
   ```

### Wiring real Drips contracts

The `pkg/drips/` package contains placeholder implementations with detailed `TODO` comments showing the expected go-ethereum binding pattern. To wire up real contract calls:

1. Generate Go bindings from Drips contract ABIs using `abigen`
2. Place generated bindings in `pkg/drips/contracts/`
3. Replace placeholder code in `streams.go` and `splits.go` with actual calls

---

## Development

```bash
make test       # Run tests with race detection
make lint       # Run golangci-lint
make run ARGS="funding status --json"
```

---

## License

MIT
