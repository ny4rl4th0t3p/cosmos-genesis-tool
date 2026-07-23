# gentool

Part of
**Seedward** — [docs](https://ny4rl4th0t3p.github.io/seedward-suite) · [ADRs](https://ny4rl4th0t3p.github.io/seedward-suite/decisions/) · [demo](https://ny4rl4th0t3p.github.io/seedward-suite/demo/)

> **Warning:** This tool is a work in progress. It has been tested and should produce correct output, but use it
> carefully on any network with real economic impact or staked value. Always validate the output genesis with your chain
> binary before launch and review the result manually.
>
> This software is provided as-is under the [Apache 2.0 License](LICENSE) with no warranty. The authors accept no
> responsibility for loss of funds, incorrect genesis state, or chain failures resulting from its use.

A CLI for generating a production-ready genesis file for any Cosmos SDK chain. It takes a baseline genesis produced by
`<chaind> init`, enriches it with validator gentxs, initial accounts, vesting claims, and vesting grants, and writes a
fully-validated output genesis.

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
2. Injects standard module accounts (`bonded_tokens_pool`, `not_bonded_tokens_pool`, `gov`, `distribution`, `mint`,
   `fee_collector`) and any chain-specific extra modules.
3. Reads all gentx files; injects validator accounts, staking validators, delegations, signing infos, and consensus
   validator set.
4. Adds non-vesting initial accounts from `accounts.csv`.
5. Adds delayed vesting accounts (**claims**) and continuous vesting accounts (**grants**); optionally pre-delegates a
   portion of claim tokens to named validators.
6. Writes final staking state (params, delegations, last validator powers), distribution state (including optional
   community pool seeding), denomination metadata, and slashing parameters.
7. Optionally seeds `authz` and `feegrant` module state from CSVs.
8. Validates total supply against `accounts.total_supply` — fails fast if the final bank supply does not match.
9. Clears `genutil.gen_txs` so the chain does not re-process gentxs on startup.
10. Sets the CometBFT consensus validator set.

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

**`claims.csv`** — delayed vesting accounts; third column is the validator moniker to pre-delegate to (optional). No
header row.

```
cosmos1def...,1000000,validator-alpha
cosmos1ghi...,1000000
```

**`grants.csv`** — continuous vesting accounts (tokens unlock linearly between `grants.vesting.start_date` and
`grants.vesting.end_date`). No header row. No delegation.

```
cosmos1jkl...,10000000000
```

**`authz.csv`** — pre-seeds the `authz` module with `GenericAuthorization` grants. Fields:
`granter, grantee, msg_type_url[, expiry_unix_timestamp]`. Expiry is optional; omit for no expiry. No header row.

```
cosmos1granter...,cosmos1grantee...,/cosmos.bank.v1beta1.MsgSend,1900000000
cosmos1granter...,cosmos1grantee...,/cosmos.staking.v1beta1.MsgDelegate
```

**`feegrant.csv`** — pre-seeds the `feegrant` module with `BasicAllowance` grants. Fields:
`granter, grantee, spend_limit_amount[, expiry_unix_timestamp]`. Set `spend_limit_amount` to `0` for no spend limit.
Expiry is optional. No header row.

```
cosmos1granter...,cosmos1grantee...,5000000,1900000000
cosmos1granter...,cosmos1grantee...,0
```

**CSV rules:**

- Amounts are in the base denomination (e.g. `uatom`). No suffix.
- Module addresses are automatically filtered from all five files.
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

The output genesis is written to the `genesis.output` path from your config. Validate it with your chain binary:

```sh
gaiad validate-genesis /path/to/output/genesis.json
```

---

## Config reference

All fields are optional unless marked **required**.

### `chain`

| Key                            | Type           | Required | Description                                                |
|--------------------------------|----------------|----------|------------------------------------------------------------|
| `chain.address_prefix`         | string         | **yes**  | Bech32 HRP (e.g. `cosmos`). Drives all address derivation. |
| `chain.id`                     | string         | **yes**  | Chain ID written to genesis metadata.                      |
| `chain.initial_height`         | int            | no       | Initial block height (default `1`).                        |
| `chain.max_validators`         | int            | no       | Staking `max_validators` param.                            |
| `chain.unbonding_time_seconds` | int            | no       | Staking `unbonding_time` in seconds.                       |
| `chain.min_commission_rate`    | decimal string | no       | Staking `min_commission_rate` (e.g. `"0.03"`).             |
| `chain.historical_entries`     | int            | no       | Staking `historical_entries` param.                        |
| `chain.max_entries`            | int            | no       | Staking `max_entries` param.                               |
| `chain.blocks_per_year`        | int            | no       | Mint `blocks_per_year` param.                              |
| `chain.inflation_rate_change`  | decimal string | no       | Mint `inflation_rate_change`.                              |
| `chain.inflation_max`          | decimal string | no       | Mint `inflation_max`.                                      |
| `chain.inflation_min`          | decimal string | no       | Mint `inflation_min`.                                      |
| `chain.goal_bonded`            | decimal string | no       | Mint `goal_bonded`.                                        |

### `app`

| Key                | Type       | Required | Description                                              |
|--------------------|------------|----------|----------------------------------------------------------|
| `app.genesis_time` | int (unix) | **yes**  | Genesis time; overrides the baseline genesis file value. |
| `app.name`         | string     | no       | Chain binary name written to genesis metadata.           |
| `app.version`      | string     | no       | App version written to genesis metadata.                 |

### Top-level

| Key                  | Type   | Required | Description                                                                 |
|----------------------|--------|----------|-----------------------------------------------------------------------------|
| `default_bond_denom` | string | **yes**  | Base staking denomination (e.g. `uatom`). Used throughout for coin parsing. |

### `slashing`

All fields fall back to the Cosmos SDK default if omitted.

| Key                                       | Type           | Description                                     |
|-------------------------------------------|----------------|-------------------------------------------------|
| `slashing.signed_blocks_window`           | int            | Window length for liveness tracking.            |
| `slashing.min_signed_per_window`          | decimal string | Minimum fraction of blocks that must be signed. |
| `slashing.downtime_jail_duration_seconds` | int            | Jail duration for downtime in seconds.          |
| `slashing.slash_fraction_double_sign`     | decimal string | Slash fraction for double-sign (e.g. `"0.05"`). |
| `slashing.slash_fraction_downtime`        | decimal string | Slash fraction for downtime (e.g. `"0.0001"`).  |

### `gov`

All fields fall back to the Cosmos SDK default if omitted.

| Key                                | Type               | Description                                         |
|------------------------------------|--------------------|-----------------------------------------------------|
| `gov.min_deposit_amount`           | int                | Minimum deposit amount in `default_bond_denom`.     |
| `gov.voting_period`                | Go duration string | Standard voting period (e.g. `"432000s"`, `"72h"`). |
| `gov.expedited_min_deposit_amount` | int                | Minimum deposit for expedited proposals.            |
| `gov.expedited_voting_period`      | Go duration string | Expedited voting period.                            |

### `denom`

Omit this entire section to preserve denom metadata from the baseline genesis. Setting `denom.base` activates the
section and overwrites all existing denom metadata.

| Key                 | Type            | Description                                           |
|---------------------|-----------------|-------------------------------------------------------|
| `denom.base`        | string          | Base denom (e.g. `uatom`).                            |
| `denom.display`     | string          | Display denom (e.g. `ATOM`).                          |
| `denom.symbol`      | string          | Ticker symbol.                                        |
| `denom.description` | string          | Human-readable description.                           |
| `denom.exponent`    | int             | Decimal exponent between base and display (e.g. `6`). |
| `denom.aliases`     | list of strings | Base denom aliases (e.g. `[microatom]`).              |

### `accounts`

| Key                          | Type   | Required | Description                                                                                                                                                                                                                                                                              |
|------------------------------|--------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `accounts.file_name`         | string | **yes**  | Path to `accounts.csv`.                                                                                                                                                                                                                                                                  |
| `accounts.total_supply`      | int    | **yes**  | True final on-chain supply in base denom. Must equal the sum of every token that will exist at genesis (see [Supply validation](#supply-validation)). Validated at runtime after all accounts are added; the tool exits with an error on mismatch.                                       |
| `accounts.non_staked_amount` | int    | no       | Absolute amount in base denom (not a percentage) kept liquid on each delegating account, so it retains a spendable balance for gas. Default `100000`. A delegating claim's amount must exceed this value or genesis creation fails. See [Vesting account types](#vesting-account-types). |

### `claims`

| Key                       | Type       | Required | Description                                          |
|---------------------------|------------|----------|------------------------------------------------------|
| `claims.file_name`        | string     | **yes**  | Path to `claims.csv`.                                |
| `claims.vesting.end_date` | int (unix) | **yes**  | Unix timestamp at which all claim tokens fully vest. |

### `grants`

| Key                         | Type       | Required | Description                                    |
|-----------------------------|------------|----------|------------------------------------------------|
| `grants.file_name`          | string     | **yes**  | Path to `grants.csv`.                          |
| `grants.vesting.start_date` | int (unix) | **yes**  | Unix timestamp at which linear vesting begins. |
| `grants.vesting.end_date`   | int (unix) | **yes**  | Unix timestamp at which vesting completes.     |

### `distribution`

All fields are optional. Omit the entire section to leave distribution state at its SDK defaults.

| Key                                  | Type | Description                                                                                                                                                                     |
|--------------------------------------|------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `distribution.community_pool_amount` | int  | Amount in `default_bond_denom` to seed into `FeePool.CommunityPool` at genesis. Added **after** supply validation (same convention as claims/grants). Default `0` (no seeding). |

### `authz`

Optional. Omit the entire section to skip authz state entirely.

| Key               | Type   | Description                                                           |
|-------------------|--------|-----------------------------------------------------------------------|
| `authz.file_name` | string | Path to `authz.csv`. Each row seeds one `GenericAuthorization` grant. |

### `feegrant`

Optional. Omit the entire section to skip feegrant state entirely.

| Key                  | Type   | Description                                                        |
|----------------------|--------|--------------------------------------------------------------------|
| `feegrant.file_name` | string | Path to `feegrant.csv`. Each row seeds one `BasicAllowance` grant. |

### `validators`

| Key                    | Type   | Required | Description                                    |
|------------------------|--------|----------|------------------------------------------------|
| `validators.gentx_dir` | string | **yes**  | Path to directory containing gentx JSON files. |

### `modules`

Optional. Injects chain-specific module accounts beyond the standard SDK set.

```yaml
modules:
  extra:
    - name: mymodule
      permissions: [ minter, burner ]
```

Valid permissions: `minter`, `burner`, `staking`.

### `genesis`

| Key              | Type   | Required | Description                             |
|------------------|--------|----------|-----------------------------------------|
| `genesis.output` | string | **yes**  | Output path for the final genesis file. |

---

## Vesting account types

**Claims** are delayed vesting accounts: all tokens vest at once at `claims.vesting.end_date`.

**Grants** are continuous vesting accounts: tokens unlock linearly from `grants.vesting.start_date` to
`grants.vesting.end_date`.

|                                     | Claims                      | Grants       |
|-------------------------------------|-----------------------------|--------------|
| CSV                                 | `claims.csv`                | `grants.csv` |
| Vesting type                        | Delayed                     | Continuous   |
| Pre-delegation                      | Optional (third CSV column) | Never        |
| Included in `accounts.total_supply` | Yes                         | Yes          |

**How claim delegation works:** when a claim row specifies a validator moniker, gentool:

1. Moves all tokens except `accounts.non_staked_amount` into the `bonded_tokens_pool` balance.
2. Keeps `non_staked_amount` liquid in the account's own balance.
3. Records a `stakingtypes.Delegation` entry so the chain tracks delegated shares from block 1.
4. Adds those shares to the validator's token weight and consensus voting power.

---

## Supply validation

`accounts.total_supply` is validated at the **end** of the pipeline, after all accounts (module accounts, initial
accounts, claims, grants, and the community pool) have been written. The bank supply at that point must equal exactly:

```
accounts.total_supply
  = sum(accounts.csv amounts)
  + sum(gentx self-delegation amounts)    ← held in bonded_tokens_pool
  + sum(claims.csv amounts)
  + sum(grants.csv amounts)
  + distribution.community_pool_amount    ← 0 if not configured
```

The tool exits with an error if the bank supply does not match this value.

---

## Module accounts

The following standard module accounts are always created automatically:

| Module                   | Initial balance                                  | Permissions         |
|--------------------------|--------------------------------------------------|---------------------|
| `bonded_tokens_pool`     | Sum of all gentx self-delegation amounts         | `burner`, `staking` |
| `not_bonded_tokens_pool` | 0                                                | `burner`, `staking` |
| `gov`                    | 0                                                | `burner`            |
| `distribution`           | `distribution.community_pool_amount` (default 0) | —                   |
| `mint`                   | 0                                                | `minter`            |
| `fee_collector`          | 0                                                | —                   |

Extra modules declared under `modules.extra` start with zero balance and whatever permissions you specify.

---

## Embedding as a library

The genesis engine is importable: `pkg/genesis` is standalone and **side-effect-limited** — its one global side effect
is sealing the process-global `sdk.Config` bech32 prefixes on the first `Build` (call it once per process with a
consistent prefix) — and `pkg/cli` exposes the commands for mounting into a host CLI.

### Mount the CLI commands

```go
import "github.com/ny4rl4th0t3p/seedward-gentool/pkg/cli"

for _, cmd := range cli.NewGenesisCommands() {
rootCmd.AddCommand(cmd)
}
```

The commands are self-contained: every flag they read (including `--config`) is declared on the commands themselves, and
config state lives in a per-command viper instance — never the global singleton a host CLI may be using.

> **Caveat:** the commands own a `--config` flag. The host CLI must not declare a persistent flag
> of the same name — cobra resolves the collision silently in favor of these commands, so the
> host's flag becomes unreachable (and never populated) for any invocation through this subtree.

### Call the engine directly

```go
import "github.com/ny4rl4th0t3p/seedward-gentool/pkg/genesis"

appGenesis, err := genesis.Build(ctx, baselineGenesisBytes, cfg, repos)
if err != nil {
return err
}
err = appGenesis.SaveAs(outputPath)
```

`Build` takes the raw baseline genesis bytes, a `genesis.ChainConfig`, and a `genesis.Repositories`
bundle. CSV- and gentx-backed repository implementations live in `pkg/genesis/csv` and
`pkg/genesis/gentx`, or you can supply your own. Only `InitialAccounts` and `Validators` are required;
`Claims`, `Grants`, `AuthzGrants`, and `FeeAllowances` are optional — leave any of them `nil` to skip those
records/modules.

---

## Memory & resource limits

gentool builds the entire genesis **in memory** — it reads the baseline genesis into a byte buffer and holds the working
state (auth accounts, bank balances, staking delegations) until the final writing. Peak RAM therefore scales with the
number of accounts/claims/grants/validators, and a huge genesis can be multi-gigabyte.

gentool does **not** try to predict or cap its own memory — bound it where it's actually enforced, at the
container/cgroup level:

```bash
docker run --memory=4g -e GOMEMLIMIT=3600MiB gentool create ...
# k8s: resources.limits.memory: 4Gi  +  env GOMEMLIMIT=3600MiB
```

- The **container memory limit** is the hard cap — exceed it and the kernel OOM-kills the process.
- [`GOMEMLIMIT`](https://pkg.go.dev/runtime#hdr-Environment_Variables) (a Go runtime env var, read automatically — no
  flag needed) is a **soft** target: as the heap approaches it, the GC runs more aggressively to stay under the hard cap
  instead of tripping it. Set it to ~**90%** of the container limit, leaving headroom for non-heap memory. It's pure Go,
  so `GOMEMLIMIT` governs essentially all of its memory.

If a genesis genuinely needs more than the limit, expect an OOM kill (or, with `GOMEMLIMIT` set, heavy GC thrashing
first) — give the job a bigger box.

---

## Roadmap

Implemented:

- [x] Community pool seeding

Planned:

- [ ] Periodic vesting accounts
- [ ] Named vesting buckets
- [x] `authz` / `feegrant` genesis grants

---

## Development

```sh
make test           # unit tests
make test-race      # unit tests with race detector
make test-cover     # coverage report (opens browser)
make test-integration  # Docker integration test (full scenario + gaiad validate-genesis)
make test-smoke     # Docker smoke test (2-validator chain boots + 35 on-chain assertions: params, supply, vesting accounts, delegations, denom metadata)
make check          # fmt + vet + tidy + lint + unit tests (CI entry point)
make fmt            # format source
make vet            # go vet
make tidy           # go mod tidy
make lint           # golangci-lint (report only)
make lint-fix       # golangci-lint with auto-fixes
```

---

## Architecture

```
cmd/gentool/          Standalone CLI entry point
pkg/
  cli/                Cobra commands + embedding API (NewGenesisCommands)
  genesis/            Importable, side-effect-limited genesis engine (seals sdk.Config on first Build)
    build.go          Build orchestrator (library entry point)
    config.go         ChainConfig
    accounts.go       Validator / claim / grant / initial account injection
    staking.go        Staking module params + delegations
    distribution.go   Distribution module state
    slashing.go       Slashing params + signing infos
    gov.go            Governance params
    mint.go           Mint params
    bank.go           Denom metadata + supply validation
    authz.go          authz genesis grants
    feegrant.go       feegrant genesis allowances
    consensus.go      CometBFT consensus validator set
    accounts/         Account domain types
    authz/            AuthzGrant domain type
    feegrant/         FeeAllowance domain type
    validator/        Validator domain type
    vestingaccount/   Claim / Grant vesting account builders
    csv/              CSV repositories (accounts, claims, grants, authz, feegrant)
    gentx/            Gentx reader (validator repository)
    encoding/         Chain-agnostic EncodingConfig + module address derivation
  config/             Shared viper → ChainConfig mapping (used by `create` and the rehearsal runner)
  rehearse/           Pre-flight rehearsal engine, embedded by seedward-rehearsal (see docs/rehearse-engine.md)
tests/
  integration/        Docker Compose: full genesis creation + gaiad validate-genesis
  smoke/              Docker Compose: 2-validator chain boots + 35 on-chain assertions (params, vesting accounts, supply, delegations, denom metadata)
```