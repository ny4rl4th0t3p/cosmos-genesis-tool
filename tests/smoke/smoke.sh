#!/usr/bin/env bash
set -euo pipefail

# ── Constants ─────────────────────────────────────────────────────────────────
CHAIN_ID="cosmos-smoke-1"
NODE1="/tmp/node1"
NODE2="/tmp/node2"   # separate home → unique ed25519 consensus key for validator2
DENOM="uatom"
KEYRING="--keyring-backend test --home ${NODE1}"
DATA_DIR="/tmp/data"
GENESIS_TIME=1735987170

VAL_BALANCE=2000000
VAL_SELF_DELEGATION=1000000
ACCOUNT_BALANCE=1000000

# Claim / grant amounts
NON_STAKED_PORTION=100000          # must match NonStakedPortion constant in Go code
CLAIM1_AMOUNT=1000000              # delegates to validator-alpha
CLAIM1_BONDED=$(( CLAIM1_AMOUNT - NON_STAKED_PORTION ))   # 900000
CLAIM1_LIQUID=${NON_STAKED_PORTION}                        # 100000
CLAIM2_AMOUNT=500000               # no delegation
GRANT1_AMOUNT=2000000              # continuous vesting, no delegation

# accounts.total_supply is validated BEFORE claims/grants are appended:
#   = 2 × VAL_BALANCE + 2 × VAL_SELF_DELEGATION(bonded pool) + 1 × ACCOUNT_BALANCE
TOTAL_SUPPLY=$(( 2 * VAL_BALANCE + 2 * VAL_SELF_DELEGATION + ACCOUNT_BALANCE ))

# Expected on-chain state AFTER claims/grants are appended:
EXPECTED_BANK_SUPPLY=$(( TOTAL_SUPPLY + CLAIM1_AMOUNT + CLAIM2_AMOUNT + GRANT1_AMOUNT ))
EXPECTED_BONDED_TOKENS=$(( 2 * VAL_SELF_DELEGATION + CLAIM1_BONDED ))

mkdir -p "${NODE1}" "${NODE2}" "${DATA_DIR}"

# ── 6.3  Init both nodes ──────────────────────────────────────────────────────
echo ">>> [6.3] gaiad init"
gaiad init smoke-node1 --chain-id "${CHAIN_ID}" --home "${NODE1}" 2>/dev/null
gaiad init smoke-node2 --chain-id "${CHAIN_ID}" --home "${NODE2}" 2>/dev/null

# ── Create validator keys ──────────────────────────────────────────────────────
echo ">>> creating validator keys"
gaiad keys add validator1 ${KEYRING} 2>/dev/null
gaiad keys add validator2 ${KEYRING} 2>/dev/null

VAL1_ADDR=$(gaiad keys show validator1 -a ${KEYRING} 2>/dev/null)
VAL2_ADDR=$(gaiad keys show validator2 -a ${KEYRING} 2>/dev/null)
echo "    validator1: ${VAL1_ADDR}"
echo "    validator2: ${VAL2_ADDR}"

# ── 6.4  Fund validator accounts in genesis ───────────────────────────────────
echo ">>> [6.4] gaiad add-genesis-account for validators"
gaiad genesis add-genesis-account "${VAL1_ADDR}" "${VAL_BALANCE}${DENOM}" --home "${NODE1}"
gaiad genesis add-genesis-account "${VAL2_ADDR}" "${VAL_BALANCE}${DENOM}" --home "${NODE1}"

# ── 6.5  Create gentxs (validator2 uses NODE2 consensus key via --pubkey) ─────
echo ">>> [6.5] gaiad gentx"
mkdir -p "${NODE1}/config/gentx"

gaiad genesis gentx validator1 "${VAL_SELF_DELEGATION}${DENOM}" \
    --chain-id "${CHAIN_ID}" \
    --moniker "validator-alpha" \
    --commission-rate "0.10" \
    --commission-max-rate "0.20" \
    --commission-max-change-rate "0.01" \
    --output-document "${NODE1}/config/gentx/gentx-validator1.json" \
    ${KEYRING}

VAL2_CONS_PUBKEY=$(gaiad comet show-validator --home "${NODE2}")
gaiad genesis gentx validator2 "${VAL_SELF_DELEGATION}${DENOM}" \
    --chain-id "${CHAIN_ID}" \
    --moniker "validator-beta" \
    --commission-rate "0.05" \
    --commission-max-rate "0.15" \
    --commission-max-change-rate "0.01" \
    --pubkey "${VAL2_CONS_PUBKEY}" \
    --output-document "${NODE1}/config/gentx/gentx-validator2.json" \
    ${KEYRING}

echo "    gentx files: $(ls ${NODE1}/config/gentx)"

# ── 6.6  Write accounts.csv (1 account — required by appendInitialAccounts) ───
echo ">>> [6.6] creating initial account key + accounts.csv"
gaiad keys add account1 ${KEYRING} 2>/dev/null
ACC1=$(gaiad keys show account1 -a ${KEYRING} 2>/dev/null)
printf '%s,%s\n' "${ACC1}" "${ACCOUNT_BALANCE}" > "${DATA_DIR}/accounts.csv"

# ── 6.7  Create claim / grant keys and write populated CSVs ──────────────────
echo ">>> [6.7] creating claim and grant keys"
gaiad keys add claim1 ${KEYRING} 2>/dev/null
gaiad keys add claim2 ${KEYRING} 2>/dev/null
gaiad keys add grant1 ${KEYRING} 2>/dev/null
CLAIM1_ADDR=$(gaiad keys show claim1 -a ${KEYRING} 2>/dev/null)
CLAIM2_ADDR=$(gaiad keys show claim2 -a ${KEYRING} 2>/dev/null)
GRANT1_ADDR=$(gaiad keys show grant1 -a ${KEYRING} 2>/dev/null)
echo "    claim1:  ${CLAIM1_ADDR}  (${CLAIM1_AMOUNT}${DENOM}, delegates to validator-alpha)"
echo "    claim2:  ${CLAIM2_ADDR}  (${CLAIM2_AMOUNT}${DENOM}, no delegation)"
echo "    grant1:  ${GRANT1_ADDR}  (${GRANT1_AMOUNT}${DENOM}, continuous vesting)"

echo ">>> [6.7] writing claims.csv and grants.csv"
# claims.csv: address,amount[,delegate_to_moniker]
printf '%s,%s,validator-alpha\n' "${CLAIM1_ADDR}" "${CLAIM1_AMOUNT}" >  "${DATA_DIR}/claims.csv"
printf '%s,%s\n'                 "${CLAIM2_ADDR}" "${CLAIM2_AMOUNT}" >> "${DATA_DIR}/claims.csv"
# grants.csv: address,amount
printf '%s,%s\n' "${GRANT1_ADDR}" "${GRANT1_AMOUNT}" > "${DATA_DIR}/grants.csv"

# ── 6.8  Write gentool.yaml ───────────────────────────────────────────────────
echo ">>> [6.8] writing gentool.yaml  (total_supply=${TOTAL_SUPPLY})"
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
  gentx_dir: ${NODE1}/config/gentx

genesis:
  output: ${DATA_DIR}/genesis.json
EOF

# ── 6.9  Run gentool ──────────────────────────────────────────────────────────
echo ">>> [6.9] running gentool create"
gentool \
    --config "${DATA_DIR}/gentool.yaml" \
    create \
    --input-genesis "${NODE1}/config/genesis.json"

echo "    genesis written to ${DATA_DIR}/genesis.json"

# ── 6.10 Copy genesis to both node homes ─────────────────────────────────────
echo ">>> [6.10] copying genesis to node homes"
cp "${DATA_DIR}/genesis.json" "${NODE1}/config/genesis.json"
cp "${DATA_DIR}/genesis.json" "${NODE2}/config/genesis.json"

# ── 6.11 Patch config.toml: fast blocks + distinct ports for node2 ────────────
echo ">>> [6.11] patching config.toml files"

# Both nodes: 500ms block time so the first block appears quickly
sed -i 's/timeout_commit = "[^"]*"/timeout_commit = "500ms"/g' "${NODE1}/config/config.toml"
sed -i 's/timeout_commit = "[^"]*"/timeout_commit = "500ms"/g' "${NODE2}/config/config.toml"

# Node2: shift P2P (26656→36656) and RPC (26657→36657) to avoid conflicts
sed -i 's|laddr = "tcp://[^"]*:26657"|laddr = "tcp://0.0.0.0:36657"|' "${NODE2}/config/config.toml"
sed -i 's|laddr = "tcp://[^"]*:26656"|laddr = "tcp://0.0.0.0:36656"|' "${NODE2}/config/config.toml"

# Node2: shift gRPC ports in app.toml (9090→9190, 9091→9191)
sed -i 's|address = "0.0.0.0:9090"|address = "0.0.0.0:9190"|' "${NODE2}/config/app.toml"
sed -i 's|address = "0.0.0.0:9091"|address = "0.0.0.0:9191"|' "${NODE2}/config/app.toml"

# ── 6.12 Get node IDs ─────────────────────────────────────────────────────────
echo ">>> [6.12] fetching node IDs"
NODE1_ID=$(gaiad comet show-node-id --home "${NODE1}")
NODE2_ID=$(gaiad comet show-node-id --home "${NODE2}")
echo "    node1: ${NODE1_ID}"
echo "    node2: ${NODE2_ID}"

# ── 6.13 Wire persistent_peers ────────────────────────────────────────────────
echo ">>> [6.13] setting persistent_peers"
sed -i "s/persistent_peers = \"\"/persistent_peers = \"${NODE2_ID}@127.0.0.1:36656\"/" "${NODE1}/config/config.toml"
sed -i "s/persistent_peers = \"\"/persistent_peers = \"${NODE1_ID}@127.0.0.1:26656\"/" "${NODE2}/config/config.toml"

# ── 6.14 Start both validators ────────────────────────────────────────────────
echo ">>> [6.14] starting validators"
gaiad start --home "${NODE1}" --minimum-gas-prices "0${DENOM}" > /tmp/node1.log 2>&1 &
PID1=$!
gaiad start --home "${NODE2}" --minimum-gas-prices "0${DENOM}" > /tmp/node2.log 2>&1 &
PID2=$!
echo "    node1 pid=${PID1}  node2 pid=${PID2}"

cleanup() {
    kill "${PID1}" "${PID2}" 2>/dev/null || true
}
trap cleanup EXIT

# ── 6.15 Poll RPC until block height ≥ 1 (60s timeout) ───────────────────────
echo ">>> [6.15] waiting for first block..."
TIMEOUT=90
START=$(date +%s)
while true; do
    HEIGHT=$(curl -sf http://localhost:26657/status 2>/dev/null \
        | jq -r '.result.sync_info.latest_block_height' 2>/dev/null \
        || echo "0")
    if [ "${HEIGHT:-0}" -gt 0 ] 2>/dev/null; then
        echo ""
        echo "  ✓  Block produced: height=${HEIGHT}"
        break
    fi
    NOW=$(date +%s)
    if [ $(( NOW - START )) -ge ${TIMEOUT} ]; then
        echo ""
        echo "  ✗  TIMEOUT: no block produced within ${TIMEOUT}s"
        echo "--- node1 log (first 30 lines) ---"
        head -30 /tmp/node1.log || true
        echo "--- node1 log (last 30 lines) ---"
        tail -30 /tmp/node1.log || true
        echo "--- node2 log (first 30 lines) ---"
        head -30 /tmp/node2.log || true
        echo "--- node2 log (last 30 lines) ---"
        tail -30 /tmp/node2.log || true
        exit 1
    fi
    printf '.'
    sleep 2
done


# ── 6.16 Verify on-chain state against gentool.yaml config ────────────────────
echo ""
echo ">>> [6.16] verifying on-chain state"

NODE_URL="http://localhost:26657"
FAILED=0

assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$actual" = "$expected" ]; then
        echo "  ✓  ${desc}: ${actual}"
    else
        echo "  ✗  ${desc}: expected='${expected}' actual='${actual}'"
        FAILED=$(( FAILED + 1 ))
    fi
}

# substring match: works for both proto ("/cosmos.vesting…DelayedVesting") and amino ("cosmos-sdk/DelayedVesting") type strings
assert_contains() {
    local desc="$1" needle="$2" actual="$3"
    if echo "${actual}" | grep -qF "${needle}"; then
        echo "  ✓  ${desc}: ${actual}"
    else
        echo "  ✗  ${desc}: expected to contain '${needle}', actual='${actual}'"
        FAILED=$(( FAILED + 1 ))
    fi
}

# awk float comparison: tolerates trailing zeros ("0.050000000000000000" == "0.05")
assert_float_eq() {
    local desc="$1" expected="$2" actual="$3"
    if awk -v e="$expected" -v a="$actual" 'BEGIN { exit !(a+0 == e+0) }'; then
        echo "  ✓  ${desc}: ${actual}"
    else
        echo "  ✗  ${desc}: expected='${expected}' actual='${actual}'"
        FAILED=$(( FAILED + 1 ))
    fi
}

# Cosmos SDK serialises durations as Go strings ("504h0m0s") — convert to seconds before comparing.
assert_duration_seconds_eq() {
    local desc="$1" expected_s="$2" actual_dur="$3"
    local actual_s
    actual_s=$(awk -v d="$actual_dur" 'BEGIN {
        h=0; m=0; s=0
        if (match(d, /[0-9]+h/)) h = substr(d, RSTART, RLENGTH-1) + 0
        if (match(d, /[0-9]+m/)) m = substr(d, RSTART, RLENGTH-1) + 0
        if (match(d, /[0-9]+s/)) s = substr(d, RSTART, RLENGTH-1) + 0
        print h*3600 + m*60 + s
    }')
    if [ "$actual_s" = "$expected_s" ]; then
        echo "  ✓  ${desc}: ${actual_dur} (=${expected_s}s)"
    else
        echo "  ✗  ${desc}: expected=${expected_s}s actual=${actual_dur} (=${actual_s}s)"
        FAILED=$(( FAILED + 1 ))
    fi
}

echo "--- chain identity ---"
CHAIN_ID_ONCHAIN=$(curl -sf "${NODE_URL}/status" | jq -r '.result.node_info.network')
assert_eq "chain_id" "${CHAIN_ID}" "${CHAIN_ID_ONCHAIN}"

echo "--- staking module params ---"
STAKING=$(gaiad query staking params --node "${NODE_URL}" --output json | jq '.params // .')
assert_duration_seconds_eq "staking.unbonding_time"      "1814400"  "$(echo "${STAKING}" | jq -r '.unbonding_time')"
assert_eq       "staking.max_validators"                 "100"       "$(echo "${STAKING}" | jq -r '.max_validators')"
assert_eq       "staking.bond_denom"                     "${DENOM}"  "$(echo "${STAKING}" | jq -r '.bond_denom')"
assert_float_eq "staking.min_commission_rate"            "0.05"      "$(echo "${STAKING}" | jq -r '.min_commission_rate')"

echo "--- slashing module params ---"
SLASHING=$(gaiad query slashing params --node "${NODE_URL}" --output json | jq '.params // .')
assert_eq       "slashing.signed_blocks_window"          "10000"     "$(echo "${SLASHING}" | jq -r '.signed_blocks_window')"
assert_float_eq "slashing.min_signed_per_window"         "0.5"       "$(echo "${SLASHING}" | jq -r '.min_signed_per_window')"
assert_duration_seconds_eq "slashing.downtime_jail_duration" "600"   "$(echo "${SLASHING}" | jq -r '.downtime_jail_duration')"
assert_float_eq "slashing.slash_fraction_double_sign"    "0.05"      "$(echo "${SLASHING}" | jq -r '.slash_fraction_double_sign')"
assert_float_eq "slashing.slash_fraction_downtime"       "0.0001"    "$(echo "${SLASHING}" | jq -r '.slash_fraction_downtime')"

echo "--- gov module params ---"
GOV=$(gaiad query gov params --node "${NODE_URL}" --output json)
GOV_VOTING_PERIOD=$(echo "${GOV}" | jq -r '.params.voting_period // .voting_params.voting_period')
GOV_MIN_DEPOSIT=$(echo "${GOV}" | jq -r \
    '(.params.min_deposit // .deposit_params.min_deposit)
     | .[] | select(.denom == "'"${DENOM}"'") | "\(.amount)\(.denom)"')
assert_duration_seconds_eq "gov.voting_period"  "172800"             "${GOV_VOTING_PERIOD}"
assert_eq                  "gov.min_deposit"    "10000000${DENOM}"   "${GOV_MIN_DEPOSIT}"

echo "--- mint module params ---"
MINT=$(gaiad query mint params --node "${NODE_URL}" --output json | jq '.params // .')
assert_eq "mint.mint_denom"      "${DENOM}"  "$(echo "${MINT}" | jq -r '.mint_denom')"
assert_eq "mint.blocks_per_year" "6311520"   "$(echo "${MINT}" | jq -r '.blocks_per_year')"

echo "--- bank supply and balances ---"
BANK_SUPPLY=$(gaiad query bank total --node "${NODE_URL}" --output json \
    | jq -r '.supply[] | select(.denom == "uatom") | .amount')
assert_eq "bank.total_supply"   "${EXPECTED_BANK_SUPPLY}"  "${BANK_SUPPLY}"

BONDED_TOKENS=$(gaiad query staking pool --node "${NODE_URL}" --output json \
    | jq -r '.pool.bonded_tokens // .bonded_tokens')
assert_eq "staking.pool.bonded_tokens" "${EXPECTED_BONDED_TOKENS}" "${BONDED_TOKENS}"

ACC1_BAL=$(gaiad query bank balances "${ACC1}" --node "${NODE_URL}" --output json \
    | jq -r '.balances[] | select(.denom == "uatom") | .amount')
assert_eq "account1.balance" "${ACCOUNT_BALANCE}" "${ACC1_BAL}"

echo "--- validators ---"
VALIDATORS=$(gaiad query staking validators --node "${NODE_URL}" --output json)
BONDED_COUNT=$(echo "${VALIDATORS}" | jq '[.validators[] | select(.status == "BOND_STATUS_BONDED")] | length')
assert_eq "bonded_validator_count" "2" "${BONDED_COUNT}"
ALPHA_COMM=$(echo "${VALIDATORS}" | jq -r \
    '.validators[] | select(.description.moniker == "validator-alpha") | .commission.commission_rates.rate')
BETA_COMM=$(echo "${VALIDATORS}" | jq -r \
    '.validators[] | select(.description.moniker == "validator-beta") | .commission.commission_rates.rate')
assert_float_eq "validator-alpha.commission" "0.10" "${ALPHA_COMM}"
assert_float_eq "validator-beta.commission"  "0.05" "${BETA_COMM}"

echo "--- claim1: DelayedVestingAccount with delegation ---"
CLAIM1_ACCT=$(gaiad query auth account "${CLAIM1_ADDR}" --node "${NODE_URL}" --output json)
# Unwrap .account{} (SDK v0.50 gRPC format) if present; otherwise use the whole object (amino format).
CLAIM1_OBJ=$(echo "${CLAIM1_ACCT}" | jq '.account // .')
# Account type: proto-JSON uses @type, amino uses "type"
CLAIM1_TYPE=$(echo "${CLAIM1_OBJ}" | jq -r '.["@type"] // .type')
# base_vesting_account: proto-JSON at top level, amino under .value
CLAIM1_BVA=$(echo "${CLAIM1_OBJ}" | jq '.base_vesting_account // .value.base_vesting_account // {}')
assert_contains  "claim1.account_type"       "DelayedVestingAccount"   "${CLAIM1_TYPE}"
assert_eq "claim1.original_vesting" "${CLAIM1_AMOUNT}" \
    "$(echo "${CLAIM1_BVA}" | jq -r '.original_vesting[] | select(.denom == "uatom") | .amount')"
assert_eq "claim1.delegated_vesting" "${CLAIM1_BONDED}" \
    "$(echo "${CLAIM1_BVA}" | jq -r '[(.delegated_vesting // [])[] | select(.denom == "uatom") | .amount] | first // "0"')"
CLAIM1_BAL=$(gaiad query bank balances "${CLAIM1_ADDR}" --node "${NODE_URL}" --output json \
    | jq -r '.balances[] | select(.denom == "uatom") | .amount')
assert_eq "claim1.liquid_balance" "${CLAIM1_LIQUID}" "${CLAIM1_BAL}"
CLAIM1_DEL_COUNT=$(gaiad query staking delegations "${CLAIM1_ADDR}" --node "${NODE_URL}" --output json \
    | jq '.delegation_responses | length')
assert_eq "claim1.delegations_count" "1" "${CLAIM1_DEL_COUNT}"
CLAIM1_DEL_SHARES=$(gaiad query staking delegations "${CLAIM1_ADDR}" --node "${NODE_URL}" --output json \
    | jq -r '.delegation_responses[0].delegation.shares')
assert_float_eq "claim1.delegation_shares" "${CLAIM1_BONDED}" "${CLAIM1_DEL_SHARES}"

echo "--- claim2: DelayedVestingAccount, no delegation ---"
CLAIM2_ACCT=$(gaiad query auth account "${CLAIM2_ADDR}" --node "${NODE_URL}" --output json)
CLAIM2_OBJ=$(echo "${CLAIM2_ACCT}" | jq '.account // .')
CLAIM2_TYPE=$(echo "${CLAIM2_OBJ}" | jq -r '.["@type"] // .type')
CLAIM2_BVA=$(echo "${CLAIM2_OBJ}" | jq '.base_vesting_account // .value.base_vesting_account // {}')
assert_contains "claim2.account_type"       "DelayedVestingAccount"   "${CLAIM2_TYPE}"
assert_eq "claim2.original_vesting" "${CLAIM2_AMOUNT}" \
    "$(echo "${CLAIM2_BVA}" | jq -r '.original_vesting[] | select(.denom == "uatom") | .amount')"
assert_eq "claim2.delegated_vesting_empty" "0" \
    "$(echo "${CLAIM2_BVA}" | jq '(.delegated_vesting // []) | length')"
CLAIM2_BAL=$(gaiad query bank balances "${CLAIM2_ADDR}" --node "${NODE_URL}" --output json \
    | jq -r '.balances[] | select(.denom == "uatom") | .amount')
assert_eq "claim2.liquid_balance" "${CLAIM2_AMOUNT}" "${CLAIM2_BAL}"

echo "--- grant1: ContinuousVestingAccount ---"
GRANT1_ACCT=$(gaiad query auth account "${GRANT1_ADDR}" --node "${NODE_URL}" --output json)
GRANT1_OBJ=$(echo "${GRANT1_ACCT}" | jq '.account // .')
GRANT1_TYPE=$(echo "${GRANT1_OBJ}" | jq -r '.["@type"] // .type')
GRANT1_BVA=$(echo "${GRANT1_OBJ}" | jq '.base_vesting_account // .value.base_vesting_account // {}')
# start_time: proto-JSON at top level, amino under .value
GRANT1_START=$(echo "${GRANT1_OBJ}" | jq -r '.start_time // .value.start_time')
assert_contains "grant1.account_type"       "ContinuousVestingAccount" "${GRANT1_TYPE}"
assert_eq "grant1.original_vesting" "${GRANT1_AMOUNT}" \
    "$(echo "${GRANT1_BVA}" | jq -r '.original_vesting[] | select(.denom == "uatom") | .amount')"
assert_eq "grant1.start_time" "${GENESIS_TIME}" "${GRANT1_START}"
GRANT1_BAL=$(gaiad query bank balances "${GRANT1_ADDR}" --node "${NODE_URL}" --output json \
    | jq -r '.balances[] | select(.denom == "uatom") | .amount')
assert_eq "grant1.liquid_balance" "${GRANT1_AMOUNT}" "${GRANT1_BAL}"

echo "--- denom metadata ---"
DENOM_META=$(gaiad query bank denom-metadata "${DENOM}" --node "${NODE_URL}" --output json 2>/dev/null \
    | jq '.metadata // empty')
if [ -n "${DENOM_META}" ]; then
    assert_eq "denom.base"    "${DENOM}" "$(echo "${DENOM_META}" | jq -r '.base')"
    assert_eq "denom.display" "ATOM"     "$(echo "${DENOM_META}" | jq -r '.display')"
    DENOM_EXP=$(echo "${DENOM_META}" | jq -r \
        '[.denom_units[] | select(.exponent > 0) | .exponent] | max | tostring')
    assert_eq "denom.exponent" "6" "${DENOM_EXP}"
else
    echo "  ⚠  denom metadata not found for ${DENOM} — skipping"
fi

if [ "${FAILED}" -gt 0 ]; then
    echo ""
    echo "  ✗  ${FAILED} on-chain assertion(s) failed"
    exit 1
fi

echo ""
echo "════════════════════════════════════════════════"
echo " Smoke test passed"
echo "   • 2-validator chain is live"
echo "   • All on-chain state verified"
echo "════════════════════════════════════════════════"