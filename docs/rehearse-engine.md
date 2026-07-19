# The rehearsal engine (`pkg/rehearse`)

`pkg/rehearse` is gentool's **pre-flight rehearsal engine** — it answers *"does this approved gentx +
allocation set actually boot and reconcile?"* before a genesis is finalized. It is consumed by
**seedward-rehearsal** (both the standalone `rehearse` CLI and the coordd-connected daemon wrap it); the
engine itself is **coordd-agnostic** and knows nothing about the coordination server.

## What it does

Given an `Input` (approved gentxs, allocation files, chain params) the engine:

1. builds a candidate genesis via `pkg/genesis`;
2. boots an ephemeral chain on **substitute validators** — the real gentx *consensus* keys are not
   available to the rehearsal runner, so it swaps in keys it controls in order to actually produce blocks;
3. runs on-chain assertions and tears the chain down;
4. returns a `Result` carrying a tri-state `Outcome`.

Because it boots substitute validators, a rehearsal is a **pre-flight on the input set** — it proves the
set boots and reconciles, but it emits **no publishable genesis** (the booted validator set isn't the real
one).

## The contract: `Input` / `Result` / `Outcome`

- **`Outcome` is tri-state** — `PASS` / `FAIL` / `ERROR`, mirroring the bridge contract coordd consumes:
  - `PASS` — the set booted and every assertion held.
  - `FAIL` — an assertion failed; the input set is bad.
  - `ERROR` — the rehearsal itself could not run (tooling / infrastructure), kept **distinct** from a
    failing set so a broken runner is never mistaken for a bad set.
- **Sentinel errors** (`errors.Is`) distinguish failure kinds for callers.

The coordd rehearsal gate ultimately keys on this `Outcome`, delivered as a signed result fact over the
bridge — see the suite bridge contract and the ADR on rehearsal being an optional bolt-on.

## Resource bounding

Like the genesis engine, the rehearse engine does not cap its own memory — bound it at the
container/cgroup level (`GOMEMLIMIT` + a memory limit).

## Possible next additions

Deferred; pulled in when a consumer needs them:

- **Surface the height reached (`blocks_advanced`).** The engine gates a `PASS` on the chain reaching
  height ≥ 1 but does not yet carry the height in `Result` — so downstream consumers (the bridge result
  fact) report `blocks_advanced: 0`. Surfacing it lets them report real progress.
- **Live per-step progress (`WithProgress`).** Stream step verdicts as they run rather than only in the
  final `Result`, so a wrapper (daemon/CLI) can relay progress live.
