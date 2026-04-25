#!/usr/bin/env bash
# Usage: validate_genesis.sh <genesis_file> <chain_id> <total_supply_pre_claims>
#                             <claim1> <claim2> <claim3>
#                             <acc1> .. <acc5>
#                             <val1_addr> <val2_addr>
#                             <chain_home>
set -euo pipefail

GENESIS_FILE="$1"
CHAIN_ID="$2"
TOTAL_SUPPLY_PRE_CLAIMS="$3"
CLAIM1="$4"
CLAIM2="$5"
CLAIM3="$6"
ACC1="$7"
ACC2="$8"
ACC3="$9"
ACC4="${10}"
ACC5="${11}"
VAL1_ADDR="${12}"
VAL2_ADDR="${13}"
CHAIN_HOME="${14}"

DENOM="uatom"
CLAIM_AMOUNT=1000000
NON_STAKED_PORTION=100000
NUM_CLAIMS=3

# Final supply = pre-claims supply + all claim amounts
EXPECTED_FINAL_SUPPLY=$(( TOTAL_SUPPLY_PRE_CLAIMS + NUM_CLAIMS * CLAIM_AMOUNT ))

# Bonded pool = 2 validator self-delegations + 2 delegating claims × (claim_amount - non_staked)
EXPECTED_BONDED=$(( 2 * 1000000 + 2 * (CLAIM_AMOUNT - NON_STAKED_PORTION) ))

PASS=0
FAIL=0

assert_eq() {
    local label="$1"
    local expected="$2"
    local actual="$3"
    if [ "${expected}" = "${actual}" ]; then
        echo "  ✓  ${label}"
        PASS=$(( PASS + 1 ))
    else
        echo "  ✗  ${label}"
        echo "       expected : ${expected}"
        echo "       actual   : ${actual}"
        FAIL=$(( FAIL + 1 ))
    fi
}

assert_contains() {
    local label="$1"
    local needle="$2"
    local haystack="$3"
    if echo "${haystack}" | grep -qF "${needle}"; then
        echo "  ✓  ${label}"
        PASS=$(( PASS + 1 ))
    else
        echo "  ✗  ${label}: '${needle}' not found"
        FAIL=$(( FAIL + 1 ))
    fi
}

assert_gt() {
    local label="$1"
    local threshold="$2"
    local actual="$3"
    if [ "${actual}" -gt "${threshold}" ]; then
        echo "  ✓  ${label} (${actual} > ${threshold})"
        PASS=$(( PASS + 1 ))
    else
        echo "  ✗  ${label}: ${actual} is not > ${threshold}"
        FAIL=$(( FAIL + 1 ))
    fi
}

echo ""
echo "════════════════════════════════════════════════════════"
echo " Validating ${GENESIS_FILE}"
echo "════════════════════════════════════════════════════════"

# ── 4.16 chain_id ─────────────────────────────────────────────────────────────
echo ""
echo "── [4.16] chain_id"
ACTUAL_CHAIN_ID=$(jq -r '.chain_id' "${GENESIS_FILE}")
assert_eq "chain_id" "${CHAIN_ID}" "${ACTUAL_CHAIN_ID}"

# ── 4.17 staking validators ───────────────────────────────────────────────────
echo ""
echo "── [4.17] staking validators"
VAL_COUNT=$(jq '.app_state.staking.validators | length' "${GENESIS_FILE}")
assert_eq "validator count" "2" "${VAL_COUNT}"

MONIKERS=$(jq -r '[.app_state.staking.validators[].description.moniker] | sort | join(",")' "${GENESIS_FILE}")
assert_eq "validator monikers" "validator-alpha,validator-beta" "${MONIKERS}"

BONDED_COUNT=$(jq '[.app_state.staking.validators[] | select(.status == "BOND_STATUS_BONDED")] | length' "${GENESIS_FILE}")
assert_eq "all validators bonded" "2" "${BONDED_COUNT}"

# ── 4.18 staking delegations ──────────────────────────────────────────────────
echo ""
echo "── [4.18] staking delegations"
DEL_COUNT=$(jq '.app_state.staking.delegations | length' "${GENESIS_FILE}")
assert_eq "delegation count (2 self + 2 claim)" "4" "${DEL_COUNT}"

# ── 4.19 bank supply ──────────────────────────────────────────────────────────
echo ""
echo "── [4.19] bank supply"
ACTUAL_SUPPLY=$(jq -r --arg denom "${DENOM}" \
    '.app_state.bank.supply[] | select(.denom == $denom) | .amount' "${GENESIS_FILE}")
assert_eq "bank supply (uatom)" "${EXPECTED_FINAL_SUPPLY}" "${ACTUAL_SUPPLY}"

# ── 4.20 bonded pool balance ──────────────────────────────────────────────────
echo ""
echo "── [4.20] bonded pool balance"
BONDED_POOL_ADDR=$(gaiad \
    --home "${CHAIN_HOME}" \
    query auth module-account bonded_tokens_pool \
    --output json 2>/dev/null \
    | jq -r '.account.base_account.address // .account.address' 2>/dev/null || echo "")

if [ -z "${BONDED_POOL_ADDR}" ]; then
    # Fallback: find the bonded pool by looking for the module account with burner+staker permissions
    BONDED_POOL_ADDR=$(jq -r '
        .app_state.auth.accounts[]
        | select(.["@type"] == "/cosmos.auth.v1beta1.ModuleAccount")
        | select(.name == "bonded_tokens_pool")
        | .base_account.address' "${GENESIS_FILE}")
fi

if [ -n "${BONDED_POOL_ADDR}" ]; then
    ACTUAL_BONDED=$(jq -r --arg addr "${BONDED_POOL_ADDR}" --arg denom "${DENOM}" \
        '.app_state.bank.balances[] | select(.address == $addr) | .coins[] | select(.denom == $denom) | .amount' \
        "${GENESIS_FILE}")
    assert_eq "bonded pool balance (uatom)" "${EXPECTED_BONDED}" "${ACTUAL_BONDED}"
else
    echo "  ✗  bonded pool address could not be determined"
    FAIL=$(( FAIL + 1 ))
fi

# ── 4.21 claim account balances ───────────────────────────────────────────────
echo ""
echo "── [4.21] claim account balances"

balance_of() {
    local addr="$1"
    jq -r --arg addr "${addr}" --arg denom "${DENOM}" \
        '.app_state.bank.balances[] | select(.address == $addr) | .coins[] | select(.denom == $denom) | .amount' \
        "${GENESIS_FILE}"
}

# claim1 and claim2 delegate — only NON_STAKED_PORTION stays in their account
CLAIM1_BAL=$(balance_of "${CLAIM1}")
assert_eq "claim1 balance (delegating, liquid portion)" "${NON_STAKED_PORTION}" "${CLAIM1_BAL}"

CLAIM2_BAL=$(balance_of "${CLAIM2}")
assert_eq "claim2 balance (delegating, liquid portion)" "${NON_STAKED_PORTION}" "${CLAIM2_BAL}"

# claim3 has no delegation — full amount stays liquid
CLAIM3_BAL=$(balance_of "${CLAIM3}")
assert_eq "claim3 balance (no delegation, full amount)" "${CLAIM_AMOUNT}" "${CLAIM3_BAL}"

# ── 4.22 auth accounts contain all expected addresses ─────────────────────────
echo ""
echo "── [4.22] auth accounts"
ALL_AUTH_ADDRS=$(jq -r '
    .app_state.auth.accounts[] |
    (
        .base_account.address //
        .base_vesting_account.base_account.address //
        .address
    ) | select(. != null)' "${GENESIS_FILE}" 2>/dev/null | sort | uniq)

for addr in "${VAL1_ADDR}" "${VAL2_ADDR}" \
            "${ACC1}" "${ACC2}" "${ACC3}" "${ACC4}" "${ACC5}" \
            "${CLAIM1}" "${CLAIM2}" "${CLAIM3}"; do
    assert_contains "auth contains ${addr:0:20}…" "${addr}" "${ALL_AUTH_ADDRS}"
done

# ── 4.23 consensus validators ─────────────────────────────────────────────────
echo ""
echo "── [4.23] consensus validators"
CONSENSUS_VAL_COUNT=$(jq '.consensus.validators | length' "${GENESIS_FILE}")
assert_gt "consensus.validators non-empty" "0" "${CONSENSUS_VAL_COUNT}"

# ── 4.24 gaiad validate-genesis ───────────────────────────────────────────────
echo ""
echo "── [4.24] gaiad validate-genesis"
if gaiad genesis validate "${GENESIS_FILE}" --home "${CHAIN_HOME}" 2>&1; then
    echo "  ✓  gaiad validate-genesis passed"
    PASS=$(( PASS + 1 ))
else
    echo "  ✗  gaiad validate-genesis failed"
    FAIL=$(( FAIL + 1 ))
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════════════════"
echo " Results: ${PASS} passed, ${FAIL} failed"
echo "════════════════════════════════════════════════════════"

if [ "${FAIL}" -gt 0 ]; then
    exit 1
fi