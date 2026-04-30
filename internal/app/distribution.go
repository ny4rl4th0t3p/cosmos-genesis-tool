package app

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/viper"
)

func (asm StateManager) setDistribution(appGenState map[string]json.RawMessage, delegations []stakingtypes.Delegation) error {
	validators, err := asm.validatorRepository.GetValidators(context.Background())
	if err != nil {
		return err
	}

	distributionGenState := distributiontypes.DefaultGenesisState()
	distributionGenState.Params = distributiontypes.Params{
		CommunityTax:        math.LegacyNewDec(0),
		BaseProposerReward:  math.LegacyNewDec(0),
		BonusProposerReward: math.LegacyNewDec(0),
		WithdrawAddrEnabled: false,
	}

	delegationsByValidator := make(map[string][]stakingtypes.Delegation, len(validators))
	for _, d := range delegations {
		delegationsByValidator[d.ValidatorAddress] = append(delegationsByValidator[d.ValidatorAddress], d)
	}

	var delegators []distributiontypes.DelegatorStartingInfoRecord
	var rewards []distributiontypes.ValidatorOutstandingRewardsRecord
	var commissions []distributiontypes.ValidatorAccumulatedCommissionRecord
	var historicalRewards []distributiontypes.ValidatorHistoricalRewardsRecord
	var currentRewards []distributiontypes.ValidatorCurrentRewardsRecord

	for i := range validators {
		delegators = append(delegators, distributiontypes.DelegatorStartingInfoRecord{
			DelegatorAddress: validators[i].DelegatorAddress(),
			ValidatorAddress: validators[i].OperatorAddress(),
			StartingInfo: distributiontypes.DelegatorStartingInfo{
				PreviousPeriod: 1,
				Stake:          math.LegacyNewDecFromInt(math.NewInt(validators[i].Amount())),
				Height:         0,
			},
		})

		var lastPeriod uint64 = 1 // period 0 is the implicit genesis period; delegators reference period ≥ 1
		historicalRewards = append(historicalRewards, distributiontypes.ValidatorHistoricalRewardsRecord{
			ValidatorAddress: validators[i].OperatorAddress(),
			Period:           lastPeriod,
			Rewards: distributiontypes.ValidatorHistoricalRewards{
				CumulativeRewardRatio: sdk.DecCoins{},
				ReferenceCount:        2,
			},
		})

		for _, d := range delegationsByValidator[validators[i].OperatorAddress()] {
			lastPeriod++
			delegators = append(delegators, distributiontypes.DelegatorStartingInfoRecord{
				DelegatorAddress: d.DelegatorAddress,
				ValidatorAddress: d.ValidatorAddress,
				StartingInfo: distributiontypes.DelegatorStartingInfo{
					PreviousPeriod: lastPeriod,
					Stake:          d.Shares,
					Height:         0,
				},
			})
			// each new delegator moves the reference to a new period, releasing the previous one
			historicalRewards[len(historicalRewards)-1].Rewards.ReferenceCount--
			historicalRewards = append(historicalRewards, distributiontypes.ValidatorHistoricalRewardsRecord{
				ValidatorAddress: validators[i].OperatorAddress(),
				Period:           lastPeriod,
				Rewards: distributiontypes.ValidatorHistoricalRewards{
					CumulativeRewardRatio: sdk.DecCoins{},
					ReferenceCount:        2,
				},
			})
		}

		rewards = append(rewards, distributiontypes.ValidatorOutstandingRewardsRecord{
			ValidatorAddress:   validators[i].OperatorAddress(),
			OutstandingRewards: sdk.DecCoins{},
		})
		commissions = append(commissions, distributiontypes.ValidatorAccumulatedCommissionRecord{
			ValidatorAddress: validators[i].OperatorAddress(),
			Accumulated:      distributiontypes.ValidatorAccumulatedCommission{Commission: sdk.DecCoins{}},
		})
		currentRewards = append(currentRewards, distributiontypes.ValidatorCurrentRewardsRecord{
			ValidatorAddress: validators[i].OperatorAddress(),
			Rewards: distributiontypes.ValidatorCurrentRewards{
				Rewards: sdk.DecCoins{},
				Period:  lastPeriod + 1,
			},
		})
	}

	distributionGenState.DelegatorStartingInfos = delegators
	distributionGenState.OutstandingRewards = rewards
	distributionGenState.ValidatorAccumulatedCommissions = commissions
	distributionGenState.ValidatorCurrentRewards = currentRewards
	distributionGenState.ValidatorHistoricalRewards = historicalRewards

	if poolAmt := viper.GetInt64("distribution.community_pool_amount"); poolAmt > 0 {
		denom := viper.GetString("default_bond_denom")
		distributionGenState.FeePool.CommunityPool = sdk.NewDecCoins(sdk.NewDecCoin(denom, math.NewInt(poolAmt)))

		hrp := viper.GetString("chain.address_prefix")
		distAddr, err := moduleAddress(hrp, "distribution")
		if err != nil {
			return err
		}
		coin := sdk.NewCoin(denom, math.NewInt(poolAmt))
		bankGenState := banktypes.GetGenesisStateFromAppState(asm.encodingConfig.Codec, appGenState)
		if err := updateBalances(
			authtypes.NewModuleAddress("distribution"),
			banktypes.Balance{Address: distAddr, Coins: sdk.NewCoins(coin)},
			sdk.NewCoins(coin),
			bankGenState,
			true,
		); err != nil {
			return fmt.Errorf("failed to update distribution bank balance: %w", err)
		}
		bankGenState.Supply = bankGenState.Supply.Add(coin)
		bankStateBz, err := asm.encodingConfig.Codec.MarshalJSON(bankGenState)
		if err != nil {
			return fmt.Errorf("failed to marshal bank genesis state: %w", err)
		}
		appGenState["bank"] = bankStateBz
	}

	// Use cdc.MarshalJSON to produce correct proto-JSON (height as "height", period as quoted int64).
	distStateBz, err := asm.encodingConfig.Codec.MarshalJSON(distributionGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal distribution genesis state: %w", err)
	}
	appGenState["distribution"] = distStateBz
	return nil
}
