#!/usr/bin/env bash
set -euo pipefail

# ── Constants ─────────────────────────────────────────────────────────────────
CHAIN_ID="cosmos-e2e-1"
CHAIN_HOME="/tmp/chain"
CHAIN_HOME2="/tmp/chain2"   # separate node home for validator2 → unique ed25519 consensus key
DENOM="uatom"
KEYRING="--keyring-backend test --home ${CHAIN_HOME}"
DATA_DIR="/tmp/data"
OUTPUT_DIR="${OUTPUT_DIR:-/data/output}"
GENESIS_TIME=1735987170

VAL_BALANCE=2000000       # each validator's genesis account balance (uatom)
VAL_SELF_DELEGATION=1000000  # each validator's gentx self-delegation (uatom)
ACCOUNT_BALANCE=5000000   # each initial account balance (uatom)
CLAIM_AMOUNT=1000000      # each claim amount (uatom)
NON_STAKED_PORTION=100000 # matches NonStakedPortion constant in Go code

NUM_VALIDATORS=2
NUM_ACCOUNTS=5
NUM_CLAIMS=3

# accounts.total_supply = true final on-chain supply (validated at end of gentool):
#   = NUM_VALIDATORS * VAL_BALANCE          (already in input genesis)
#   + NUM_VALIDATORS * VAL_SELF_DELEGATION  (bonded pool, appendModuleAccounts)
#   + NUM_ACCOUNTS   * ACCOUNT_BALANCE      (appendInitialAccounts)
#   + NUM_CLAIMS     * CLAIM_AMOUNT         (claims, appendVestingAccounts)
TOTAL_SUPPLY=$(( NUM_VALIDATORS * VAL_BALANCE \
               + NUM_VALIDATORS * VAL_SELF_DELEGATION \
               + NUM_ACCOUNTS   * ACCOUNT_BALANCE \
               + NUM_CLAIMS     * CLAIM_AMOUNT ))

mkdir -p "${CHAIN_HOME}" "${CHAIN_HOME2}" "${DATA_DIR}" "${OUTPUT_DIR}"

# ── 4.5  Init chain ───────────────────────────────────────────────────────────
echo ">>> [4.5] gaiad init"
gaiad init e2e-test-node  --chain-id "${CHAIN_ID}" --home "${CHAIN_HOME}"  2>/dev/null
# second init gives validator2 a unique priv_validator_key.json (distinct consensus key)
gaiad init e2e-test-node2 --chain-id "${CHAIN_ID}" --home "${CHAIN_HOME2}" 2>/dev/null

# ── 4.6  Create 2 validator keys; capture addresses ──────────────────────────
echo ">>> [4.6] creating validator keys"
gaiad keys add validator1 ${KEYRING} 2>/dev/null
gaiad keys add validator2 ${KEYRING} 2>/dev/null

VAL1_ADDR=$(gaiad keys show validator1 -a ${KEYRING} 2>/dev/null)
VAL2_ADDR=$(gaiad keys show validator2 -a ${KEYRING} 2>/dev/null)
echo "    validator1: ${VAL1_ADDR}"
echo "    validator2: ${VAL2_ADDR}"

# ── 4.7  Fund validator accounts in genesis ───────────────────────────────────
echo ">>> [4.7] gaiad add-genesis-account for validators"
gaiad genesis add-genesis-account "${VAL1_ADDR}" "${VAL_BALANCE}${DENOM}" --home "${CHAIN_HOME}"
gaiad genesis add-genesis-account "${VAL2_ADDR}" "${VAL_BALANCE}${DENOM}" --home "${CHAIN_HOME}"

# ── 4.8  Create gentxs ────────────────────────────────────────────────────────
echo ">>> [4.8] gaiad gentx"
mkdir -p "${CHAIN_HOME}/config/gentx"
gaiad genesis gentx validator1 "${VAL_SELF_DELEGATION}${DENOM}" \
    --chain-id "${CHAIN_ID}" \
    --moniker "validator-alpha" \
    --commission-rate "0.10" \
    --commission-max-rate "0.20" \
    --commission-max-change-rate "0.01" \
    --output-document "${CHAIN_HOME}/config/gentx/gentx-validator1.json" \
    ${KEYRING}

VAL2_CONS_PUBKEY=$(gaiad comet show-validator --home "${CHAIN_HOME2}")
gaiad genesis gentx validator2 "${VAL_SELF_DELEGATION}${DENOM}" \
    --chain-id "${CHAIN_ID}" \
    --moniker "validator-beta" \
    --commission-rate "0.05" \
    --commission-max-rate "0.15" \
    --commission-max-change-rate "0.01" \
    --pubkey "${VAL2_CONS_PUBKEY}" \
    --output-document "${CHAIN_HOME}/config/gentx/gentx-validator2.json" \
    ${KEYRING}

GENTX_DIR="${CHAIN_HOME}/config/gentx"
echo "    gentx files: $(ls ${GENTX_DIR})"

# ── 4.9  Create 5 initial account keys ───────────────────────────────────────
echo ">>> [4.9] creating account keys"
for i in $(seq 1 ${NUM_ACCOUNTS}); do
    gaiad keys add "account${i}" ${KEYRING} 2>/dev/null
done

ACC1=$(gaiad keys show account1 -a ${KEYRING} 2>/dev/null)
ACC2=$(gaiad keys show account2 -a ${KEYRING} 2>/dev/null)
ACC3=$(gaiad keys show account3 -a ${KEYRING} 2>/dev/null)
ACC4=$(gaiad keys show account4 -a ${KEYRING} 2>/dev/null)
ACC5=$(gaiad keys show account5 -a ${KEYRING} 2>/dev/null)

# ── 4.10 Create 3 claim keys ──────────────────────────────────────────────────
echo ">>> [4.10] creating claim keys"
for i in $(seq 1 ${NUM_CLAIMS}); do
    gaiad keys add "claim${i}" ${KEYRING} 2>/dev/null
done

CLAIM1=$(gaiad keys show claim1 -a ${KEYRING} 2>/dev/null)
CLAIM2=$(gaiad keys show claim2 -a ${KEYRING} 2>/dev/null)
CLAIM3=$(gaiad keys show claim3 -a ${KEYRING} 2>/dev/null)

# ── 4.11 Write accounts.csv ───────────────────────────────────────────────────
# Validators are NOT listed here — they are already present in the input genesis
# via `gaiad add-genesis-account` above.  Including them would cause
# `genutil.AddGenesisAccount(append=false)` to fail with "account already exists".
echo ">>> [4.11] writing accounts.csv"
cat > "${DATA_DIR}/accounts.csv" <<EOF
${ACC1},${ACCOUNT_BALANCE}
${ACC2},${ACCOUNT_BALANCE}
${ACC3},${ACCOUNT_BALANCE}
${ACC4},${ACCOUNT_BALANCE}
${ACC5},${ACCOUNT_BALANCE}
EOF

# ── 4.12 Write claims.csv ─────────────────────────────────────────────────────
# Format: address,amount[,validator_moniker]
# claim1 → delegates to validator-alpha: 900 000 bonded + 100 000 liquid
# claim2 → delegates to validator-beta:  900 000 bonded + 100 000 liquid
# claim3 → no delegation: full 1 000 000 liquid
echo ">>> [4.12] writing claims.csv"
printf '%s,%s,%s\n' "${CLAIM1}" "${CLAIM_AMOUNT}" "validator-alpha" >  "${DATA_DIR}/claims.csv"
printf '%s,%s,%s\n' "${CLAIM2}" "${CLAIM_AMOUNT}" "validator-beta"  >> "${DATA_DIR}/claims.csv"
printf '%s,%s\n'    "${CLAIM3}" "${CLAIM_AMOUNT}"                   >> "${DATA_DIR}/claims.csv"

# ── 4.13 Write empty grants.csv ───────────────────────────────────────────────
# appendGrants is defined but not wired into SetupAppState — grants have no
# effect in the current pipeline.  The file must exist so the repository opens
# without error.
echo ">>> [4.13] writing empty grants.csv"
touch "${DATA_DIR}/grants.csv"

# ── 4.14 Write gentool.yaml ───────────────────────────────────────────────────
echo ">>> [4.14] writing gentool.yaml  (total_supply=${TOTAL_SUPPLY})"
cat > "${DATA_DIR}/gentool.yaml" <<EOF
chain:
  id: ${CHAIN_ID}
  address_prefix: cosmos
  initial_height: 1
  max_validators: 100
  blocks_per_year: 6311520
  unbonding_time_seconds: 1814400
  min_commission_rate: "0.05"
  historical_entries: 10000
  max_entries: 7

default_bond_denom: ${DENOM}

app:
  version: "0.0.1"
  name: gaia
  genesis_time: ${GENESIS_TIME}

slashing:
  signed_blocks_window: 10000
  min_signed_per_window: "0.5"
  downtime_jail_duration_seconds: 600
  slash_fraction_double_sign: "0.05"
  slash_fraction_downtime: "0.0001"

gov:
  min_deposit_amount: 10000000
  voting_period: "172800s"
  expedited_min_deposit_amount: 50000000
  expedited_voting_period: "86400s"

denom:
  base: ${DENOM}
  display: ATOM
  description: "The native staking token of the Cosmos Hub"
  symbol: ATOM
  exponent: 6
  aliases: [microatom]

claims:
  file_name: ${DATA_DIR}/claims.csv
  vesting:
    end_date: 1900000000

grants:
  file_name: ${DATA_DIR}/grants.csv
  vesting:
    start_date: ${GENESIS_TIME}
    end_date: 1900000000

accounts:
  total_supply: ${TOTAL_SUPPLY}
  file_name: ${DATA_DIR}/accounts.csv

validators:
  gentx_dir: ${GENTX_DIR}

genesis:
  output: ${OUTPUT_DIR}/genesis.json
EOF

# ── 4.15 Run gentool ──────────────────────────────────────────────────────────
echo ">>> [4.15] running gentool create"
gentool \
    --config "${DATA_DIR}/gentool.yaml" \
    create \
    --input-genesis "${CHAIN_HOME}/config/genesis.json"

echo "    genesis written to ${OUTPUT_DIR}/genesis.json"

# ── Validate ──────────────────────────────────────────────────────────────────
/validate_genesis.sh \
    "${OUTPUT_DIR}/genesis.json" \
    "${CHAIN_ID}" \
    "${TOTAL_SUPPLY}" \
    "${CLAIM1}" "${CLAIM2}" "${CLAIM3}" \
    "${ACC1}" "${ACC2}" "${ACC3}" "${ACC4}" "${ACC5}" \
    "${VAL1_ADDR}" "${VAL2_ADDR}" \
    "${CHAIN_HOME}"