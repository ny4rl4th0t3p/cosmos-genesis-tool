# gentool — design decisions

Repo-internal decisions for `seedward-gentool`. The cross-cutting ones — the library-first engine,
mountable command tree, and the rehearsal engine — live in the suite ADR log. This file records choices
that only matter inside this repo. Distilled from the project's build history.

## Domain types are flat sibling packages under `pkg/genesis` (2026-06-08)

Account / validator / vesting / authz / feegrant domain types live as flat siblings
(`pkg/genesis/accounts`, `.../validator`, `.../vestingaccount`, …) rather than under a `domain/` layer —
idiomatic Go grouping. They are public import paths; the grouping is the internal choice.

## `non_staked_portion` → `non_staked_amount`: an absolute reserve, not a percentage (2026-06-08)

The liquid balance kept on each delegating account is an **absolute amount** in base denom (default
`100000`), hard-renamed from `non_staked_portion` with **no alias**. A delegating claim's amount must
exceed the reserve or genesis creation hard-errors; the reserve floor is not configurable down to `0`.

## Strict supply validation at the end of the pipeline (2026-06-08)

`accounts.total_supply` is the true final on-chain supply, validated **after** all accounts, claims,
grants, and the community pool are written — a mismatch is a fast, hard error. The orphaned
`InitialAccount.IsInRemainderAllowedList` / `accounts.remainder_allowlist` was deleted rather than kept
as dead config.

## CSV is the only allocation format (2026-06)

TSV support was dropped; allocation inputs (accounts, claims, grants, authz, feegrant) are CSV only.

## Generalized; input genesis + address prefix are required (2026-06)

No chain-specific assumptions: `--input-genesis` and `chain.address_prefix` are **required**, module addresses are
derived dynamically from the prefix, and all raw-byte genesis patches were replaced with `cdc.MarshalJSON`.

## `--config` is owned by the `create` command; per-command viper (2026-06-11)

`create` owns the `--config` flag and the root aliases the same `*pflag.Flag`. A per-command `viper.New()`
replaces the global viper + `cobra.OnInitialize`, so the commands carry no global state — the precondition
for mounting them into a host CLI (the suite ADR on the mountable command tree).
