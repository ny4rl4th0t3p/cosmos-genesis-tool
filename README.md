# gentool

> **Warning:** This tool is a work in progress. It has been tested and should produce correct output, but use it carefully on any network with real economic impact or staked value. Always validate the output genesis with your chain binary before launch and review the result manually.
>
> This software is provided as-is under the [Apache 2.0 License](LICENSE) with no warranty. The authors accept no responsibility for loss of funds, incorrect genesis state, or chain failures resulting from its use.

A CLI for generating a production-ready genesis file for any Cosmos SDK chain.
It takes a baseline genesis produced by `<chaind> init`, enriches it with
validator gentxs, initial accounts, vesting claims, and vesting grants, and
writes a fully-validated output genesis.

---

## How it works

```
<chaind> init  →  baseline genesis.json
                        │
              gentool create
                        │
         ┌──────────────┼──────────────┐
    gentx dir      accounts.csv    claims.csv / grants.csv
         │              │               │
         └──────────────┴───────────────┘
                        │
                  output genesis.json
```

The tool runs these steps in order:

1. Applies governance and mint parameters from config.
2. Injects standard module accounts (`bonded_tokens_pool`, `not_bonded_tokens_pool`, `gov`, `distribution`, `mint`, `fee_collector`) and any chain-specific extra modules.
3. Reads all gentx files; injects validator accounts, staking validators, delegations, signing infos, and consensus validator set.
4. Adds non-vesting initial accounts from `accounts.csv`.
5. Validates total supply against `accounts.total_supply` — fails fast if mismatched.
6. Adds delayed vesting accounts (**claims**) and continuous vesting accounts (**grants**); optionally pre-delegates a portion of claim tokens to named validators.
7. Writes final staking state (params, delegations, last validator powers), distribution state, denomination metadata, and slashing parameters.
8. Clears `genutil.gen_txs` so the chain does not re-process gentxs on startup.
9. Sets the CometBFT consensus validator set.

---

## Prerequisites

- Go 1.25+
- A chain binary (e.g. `gaiad`) to produce the baseline genesis and gentxs

---

## Build

```sh
make build          # → build/gentool
make install        # → $(GOPATH)/bin/gentool
```

---

## Workflow

### 1. Produce a baseline genesis

```sh
gaiad init <moniker> --chain-id <chain-id>
```

### 2. Collect validator gentxs

Each validator runs:

```sh
gaiad genesis gentx <key-name> <self-delegation-amount>uatom \
  --chain-id <chain-id> \
  --moniker <moniker>
```

Gather the resulting `~/.gaiad/config/gentx/*.json` files into a single directory.

### 3. Prepare input CSVs

**`accounts.csv`** — non-vesting initial accounts, one per line. No header row.

```
cosmos1abc...,5000000000
cosmos1def...,1000000000
```

**`claims.csv`** — delayed vesting accounts; third column is the validator moniker to pre-delegate to (optional). No header row.

```
cosmos1def...,1000000,validator-alpha
cosmos1ghi...,1000000
```

**`grants.csv`** — continuous vesting accounts (tokens unlock linearly between `grants.vesting.start_date` and `grants.vesting.end_date`). No header row. No delegation.

```
cosmos1jkl...,10000000000
```

**CSV rules:**
- Amounts are in the base denomination (e.g. `uatom`). No suffix.
- Module addresses are automatically filtered from all three files.
- Accounts with amount `0` in `accounts.csv` are skipped.
- No header row in any file.
- Leading/trailing whitespace is stripped from each field.

### 4. Write a config file

```sh
cp .gentool.yaml.example gentool.yaml
```

### 5. Run

```sh
gentool create \
  --input-genesis ~/.gaiad/config/genesis.json \
  --config gentool.yaml
```

The output genesis is written to `genesis.output` from your config. Validate with:

```sh
gaiad validate-genesis $TMPDIR/gentool/genesis.json
```

---

## Config reference

All fields are optional unless marked **required**.

### `chain`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `chain.address_prefix` | string | **yes** | Bech32 HRP (e.g. `cosmos`). Drives all address derivation. |
| `chain.id` | string | **yes** | Chain ID written to genesis metadata. |
| `chain.initial_height` | int | no | Initial block height (default `1`). |
| `chain.max_validators` | int | no | Staking `max_validators` param. |
| `chain.unbonding_time_seconds` | int | no | Staking `unbonding_time` in seconds. |
| `chain.min_commission_rate` | decimal string | no | Staking `min_commission_rate` (e.g. `"0.03"`). |
| `chain.historical_entries` | int | no | Staking `historical_entries` param. |
| `chain.max_entries` | int | no | Staking `max_entries` param. |
| `chain.blocks_per_year` | int | no | Mint `blocks_per_year` param. |
| `chain.inflation_rate_change` | decimal string | no | Mint `inflation_rate_change`. |
| `chain.inflation_max` | decimal string | no | Mint `inflation_max`. |
| `chain.inflation_min` | decimal string | no | Mint `inflation_min`. |
| `chain.goal_bonded` | decimal string | no | Mint `goal_bonded`. |

### `app`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `app.genesis_time` | int (unix) | **yes** | Genesis time; overrides the baseline genesis file value. |
| `app.name` | string | no | Chain binary name written to genesis metadata. |
| `app.version` | string | no | App version written to genesis metadata. |

### Top-level

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `default_bond_denom` | string | **yes** | Base staking denomination (e.g. `uatom`). Used throughout for coin parsing. |

### `slashing`

All fields fall back to the Cosmos SDK default if omitted.

| Key | Type | Description |
|-----|------|-------------|
| `slashing.signed_blocks_window` | int | Window length for liveness tracking. |
| `slashing.min_signed_per_window` | decimal string | Minimum fraction of blocks that must be signed. |
| `slashing.downtime_jail_duration_seconds` | int | Jail duration for downtime in seconds. |
| `slashing.slash_fraction_double_sign` | decimal string | Slash fraction for double-sign (e.g. `"0.05"`). |
| `slashing.slash_fraction_downtime` | decimal string | Slash fraction for downtime (e.g. `"0.0001"`). |

### `gov`

All fields fall back to the Cosmos SDK default if omitted.

| Key | Type | Description |
|-----|------|-------------|
| `gov.min_deposit_amount` | int | Minimum deposit amount in `default_bond_denom`. |
| `gov.voting_period` | Go duration string | Standard voting period (e.g. `"432000s"`, `"72h"`). |
| `gov.expedited_min_deposit_amount` | int | Minimum deposit for expedited proposals. |
| `gov.expedited_voting_period` | Go duration string | Expedited voting period. |

### `denom`

Omit this entire section to preserve denom metadata from the baseline genesis. Setting `denom.base` activates the section and overwrites all existing denom metadata.

| Key | Type | Description |
|-----|------|-------------|
| `denom.base` | string | Base denom (e.g. `uatom`). |
| `denom.display` | string | Display denom (e.g. `ATOM`). |
| `denom.symbol` | string | Ticker symbol. |
| `denom.description` | string | Human-readable description. |
| `denom.exponent` | int | Decimal exponent between base and display (e.g. `6`). |
| `denom.aliases` | list of strings | Base denom aliases (e.g. `[microatom]`). |

### `accounts`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `accounts.file_name` | string | **yes** | Path to `accounts.csv`. |
| `accounts.total_supply` | int | **yes** | Expected total supply in base denom, counted before claims/grants are added. Must equal `sum(accounts.csv) + sum(gentx self-delegations)`. Validated at runtime; the tool exits with an error on mismatch. |
| `accounts.non_staked_portion` | int | no | Amount in base denom kept liquid per delegating claim. Default `100000`. See [Vesting account types](#vesting-account-types). |

### `claims`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `claims.file_name` | string | **yes** | Path to `claims.csv`. |
| `claims.vesting.end_date` | int (unix) | **yes** | Unix timestamp at which all claim tokens fully vest. |

### `grants`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `grants.file_name` | string | **yes** | Path to `grants.csv`. |
| `grants.vesting.start_date` | int (unix) | **yes** | Unix timestamp at which linear vesting begins. |
| `grants.vesting.end_date` | int (unix) | **yes** | Unix timestamp at which vesting completes. |

### `validators`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `validators.gentx_dir` | string | **yes** | Path to directory containing gentx JSON files. |

### `modules`

Optional. Injects chain-specific module accounts beyond the standard SDK set.

```yaml
modules:
  extra:
    - name: mymodule
      permissions: [minter, burner]
```

Valid permissions: `minter`, `burner`, `staking`.

### `genesis`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `genesis.output` | string | **yes** | Output path for the final genesis file. |

---

## Vesting account types

**Claims** are delayed vesting accounts: all tokens vest at once at `claims.vesting.end_date`.

**Grants** are continuous vesting accounts: tokens unlock linearly from `grants.vesting.start_date` to `grants.vesting.end_date`.

| | Claims | Grants |
|-|--------|--------|
| CSV | `claims.csv` | `grants.csv` |
| Vesting type | Delayed | Continuous |
| Pre-delegation | Optional (third CSV column) | Never |
| Adds to `total_supply` | No — added after supply check | No — added after supply check |

**How claim delegation works:** when a claim row specifies a validator moniker, gentool:

1. Moves all tokens except `accounts.non_staked_portion` into the `bonded_tokens_pool` balance.
2. Keeps `non_staked_portion` liquid in the account's own balance.
3. Records a `stakingtypes.Delegation` entry so the chain tracks delegated shares from block 1.
4. Adds those shares to the validator's token weight and consensus voting power.

---

## Supply validation

`accounts.total_supply` is checked after module accounts and initial accounts are written, but **before** claims and grants are added. At that checkpoint the bank supply must equal exactly:

```
accounts.total_supply
  = sum(accounts.csv amounts)
  + sum(gentx self-delegation amounts)    ← held in bonded_tokens_pool
```

Claims and grants each contribute their own token amounts on top of this figure and are not included in `accounts.total_supply`.

---

## Module accounts

The following standard module accounts are always created automatically:

| Module | Initial balance | Permissions |
|--------|----------------|-------------|
| `bonded_tokens_pool` | Sum of all gentx self-delegation amounts | `burner`, `staking` |
| `not_bonded_tokens_pool` | 0 | `burner`, `staking` |
| `gov` | 0 | `burner` |
| `distribution` | 0 | — |
| `mint` | 0 | `minter` |
| `fee_collector` | 0 | — |

Extra modules declared under `modules.extra` start with zero balance and whatever permissions you specify.

---

## Roadmap

The following features are planned but not yet implemented:

- [ ] Periodic vesting accounts
- [ ] Named vesting buckets
- [ ] `authz` / `feegrant` genesis grants
- [ ] Community pool seeding

---

## Development

```sh
make test           # unit tests
make test-race      # unit tests with race detector
make test-cover     # coverage report (opens browser)
make test-integration  # Docker integration test (full scenario + gaiad validate-genesis)
make test-smoke     # Docker smoke test (2-validator chain boots + 35 on-chain assertions: params, supply, vesting accounts, delegations, denom metadata)
make check          # fmt + vet + tidy + unit tests (CI entry point)
make fmt            # format source
make vet            # go vet
make tidy           # go mod tidy
```

---

## Architecture

```
cmd/gentool/          CLI entry point (Cobra)
internal/
  app/                Genesis construction logic
    app_state.go      SetupAppState orchestrator
    accounts.go       Validator / claim / grant / initial account injection
    staking.go        Staking module params + delegations
    distribution.go   Distribution module state
    slashing.go       Slashing params + signing infos
    gov.go            Governance params
    mint.go           Mint params
    bank.go           Denom metadata + supply validation
    consensus.go      CometBFT consensus validator set
    utils.go          Shared helpers (LoadGenesis, updateModuleState, vesting account builders)
  domain/             Pure domain types (Validator, Claim, Grant, InitialAccount)
  encoding/           Chain-agnostic EncodingConfig
  repository/         CSV + gentx readers
tests/
  integration/        Docker Compose: full genesis creation + gaiad validate-genesis
  smoke/              Docker Compose: 2-validator chain boots + 35 on-chain assertions (params, vesting accounts, supply, delegations, denom metadata)
```