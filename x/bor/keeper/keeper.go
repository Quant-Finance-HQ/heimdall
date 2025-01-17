package keeper

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"

	checkpointKeeper "github.com/maticnetwork/heimdall/x/checkpoint/keeper"

	"github.com/maticnetwork/bor/common"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/maticnetwork/heimdall/helper"
	"github.com/maticnetwork/heimdall/merr"
	hmTypes "github.com/maticnetwork/heimdall/types"
	"github.com/maticnetwork/heimdall/x/bor/types"
	chainManagerKeeper "github.com/maticnetwork/heimdall/x/chainmanager/keeper"
	stakingKeeper "github.com/maticnetwork/heimdall/x/staking/keeper"
)

var (
	DefaultValue = []byte{0x01} // Value to store in CacheCheckpoint and CacheCheckpointACK & ValidatorSetChange Flag

	SpanDurationKey       = []byte{0x24} // Key to store span duration for Bor
	SprintDurationKey     = []byte{0x25} // Key to store span duration for Bor
	LastSpanIDKey         = []byte{0x35} // Key to store last span start block
	SpanPrefixKey         = []byte{0x36} // prefix key to store span
	SpanCacheKey          = []byte{0x37} // key to store Cache for span
	LastProcessedEthBlock = []byte{0x38} // key to store last processed eth block for seed
)

// Keeper stores all related data
type Keeper struct {
	cdc codec.BinaryMarshaler
	sk  stakingKeeper.Keeper
	// The (unexposed) keys used to access the stores from the Context.
	storeKey sdk.StoreKey
	// param space
	paramSpace paramtypes.Subspace
	// contract caller
	ContractCaller helper.ContractCaller
	// chain manager keeper
	chainKeeper chainManagerKeeper.Keeper
	// checkpoint keeper
	checkpointKeeper checkpointKeeper.Keeper
}

// NewKeeper create new keeper
func NewKeeper(
	cdc codec.BinaryMarshaler,
	storeKey sdk.StoreKey,
	paramSubspace paramtypes.Subspace,
	chainKeeper chainManagerKeeper.Keeper,
	stakingKeeper stakingKeeper.Keeper,
	checkpointKeeper checkpointKeeper.Keeper,
	caller helper.ContractCaller,
) Keeper {
	// create keeper
	if !paramSubspace.HasKeyTable() {
		paramSubspace = paramSubspace.WithKeyTable(types.ParamKeyTable())
	}
	keeper := Keeper{
		cdc:              cdc,
		storeKey:         storeKey,
		paramSpace:       paramSubspace,
		chainKeeper:      chainKeeper,
		sk:               stakingKeeper,
		checkpointKeeper: checkpointKeeper,
		ContractCaller:   caller,
	}
	return keeper
}

// Logger returns a module-specific logger
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// -----------------------------------------------------------------------------
// Params

// SetParams sets the bor module's parameters.
func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) {
	k.paramSpace.SetParamSet(ctx, params)
}

// GetParams gets the bor module's parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return
}

// GetSpanKey appends prefix to start block
func GetSpanKey(id uint64) []byte {
	return append(SpanPrefixKey, []byte(strconv.FormatUint(id, 10))...)
}

// AddNewSpan adds new span for bor to store
func (k *Keeper) AddNewSpan(ctx sdk.Context, span hmTypes.Span) error {
	store := ctx.KVStore(k.storeKey)
	out, err := k.cdc.MarshalBinaryBare(&span)
	if err != nil {
		k.Logger(ctx).Error("Error marshalling span", "error", err)
		return err
	}

	spanKey := GetSpanKey(span.ID)
	if spanKey == nil {
		k.Logger(ctx).Error("Error invalid span key")
		return merr.ValErr{Field: "span key", Module: types.ModuleName}

	}
	// store set span id
	store.Set(spanKey, out)

	// update last span
	k.UpdateLastSpan(ctx, span.ID)
	return nil
}

// AddNewRawSpan adds new span for bor to store
func (k *Keeper) AddNewRawSpan(ctx sdk.Context, span hmTypes.Span) error {
	store := ctx.KVStore(k.storeKey)
	out, err := k.cdc.MarshalBinaryBare(&span)
	if err != nil {
		k.Logger(ctx).Error("Error marshalling span", "error", err)
		return err
	}
	store.Set(GetSpanKey(span.ID), out)
	return nil
}

// GetSpan fetches span indexed by id from store
func (k *Keeper) GetSpan(ctx sdk.Context, id uint64) (*hmTypes.Span, error) {
	store := ctx.KVStore(k.storeKey)
	spanKey := GetSpanKey(id)

	// If we are starting from 0 there will be no spanKey present
	if !store.Has(spanKey) {
		return nil, errors.New("span not found for id")
	}

	var span hmTypes.Span
	if err := k.cdc.UnmarshalBinaryBare(store.Get(spanKey), &span); err != nil {
		k.Logger(ctx).Error("Error unmarshalling span", "error", err)
		return nil, err
	}

	return &span, nil
}

func (k *Keeper) HasSpan(ctx sdk.Context, id uint64) bool {
	store := ctx.KVStore(k.storeKey)
	spanKey := GetSpanKey(id)
	return store.Has(spanKey)
}

// GetAllSpans fetches all indexed by id from store
func (k *Keeper) GetAllSpans(ctx sdk.Context) ([]*hmTypes.Span, error) {
	var spansList []*hmTypes.Span
	// iterate through spans and create span update array
	err := k.IterateSpansAndApplyFn(ctx, func(span hmTypes.Span) error {
		// append to list of validatorUpdates
		spansList = append(spansList, &span)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return spansList, err
}

// GetSpanList returns all spans with params like page and limit
func (k *Keeper) GetSpanList(ctx sdk.Context, page uint64, limit uint64) ([]*hmTypes.Span, error) {
	store := ctx.KVStore(k.storeKey)

	// create spans
	var spans []*hmTypes.Span

	// have max limit
	if limit > 20 {
		limit = 20
	}

	// get paginated iterator
	iterator := hmTypes.KVStorePrefixIteratorPaginated(store, SpanPrefixKey, uint(page), uint(limit))

	// loop through validators to get valid validators
	for ; iterator.Valid(); iterator.Next() {
		var span hmTypes.Span
		err := k.cdc.UnmarshalBinaryBare(iterator.Value(), &span)
		if err == nil {
			spans = append(spans, &span)
		} else {
			k.Logger(ctx).Error("Error unmarshalling span", "error", err)
			return nil, err
		}
	}

	return spans, nil
}

// GetLastSpan fetches last span using lastStartBlock
func (k *Keeper) GetLastSpan(ctx sdk.Context) (*hmTypes.Span, error) {
	store := ctx.KVStore(k.storeKey)

	var lastSpanID uint64
	if store.Has(LastSpanIDKey) {
		// get last span id
		var err error
		lastSpanID, err = strconv.ParseUint(string(store.Get(LastSpanIDKey)), 10, 64)
		if err != nil {
			return nil, err
		}
	}

	return k.GetSpan(ctx, lastSpanID)
}

// FreezeSet freezes validator set for next span
func (k *Keeper) FreezeSet(ctx sdk.Context, id uint64, startBlock uint64, endBlock uint64, borChainID string, seed common.Hash) error {

	// select next producers
	newProducers, err := k.SelectNextProducers(ctx, seed)
	if err != nil {
		return err
	}

	// increment last eth block
	k.IncrementLastEthBlock(ctx)

	validatorSet := k.sk.GetValidatorSet(ctx)

	vSet := hmTypes.ValidatorSet{
		Validators:       validatorSet.Validators,
		TotalVotingPower: validatorSet.TotalVotingPower,
		Proposer:         validatorSet.Proposer,
	}

	// generate new span
	newSpan := hmTypes.NewSpan(
		id,
		startBlock,
		endBlock,
		vSet,
		newProducers,
		borChainID,
	)

	return k.AddNewSpan(ctx, newSpan)
}

// SelectNextProducers selects producers for next span
func (k *Keeper) SelectNextProducers(ctx sdk.Context, seed common.Hash) (vals []hmTypes.Validator, err error) {
	// spanEligibleVals are current validators who are not getting deactivated in between next span
	spanEligibleVals := k.sk.GetSpanEligibleValidators(ctx)
	producerCount := k.GetParams(ctx).ProducerCount

	// if producers to be selected is more than current validators no need to select/shuffle
	if len(spanEligibleVals) <= int(producerCount) {
		return spanEligibleVals, nil
	}

	// select next producers using seed as blockheader hash
	newProducersIds, err := SelectNextProducers(seed, spanEligibleVals, producerCount)
	if err != nil {
		return vals, err
	}

	IDToPower := make(map[uint64]uint64)
	for _, ID := range newProducersIds {
		IDToPower[ID] += 1
	}

	for key, value := range IDToPower {
		if val, ok := k.sk.GetValidatorFromValID(ctx, hmTypes.NewValidatorID(key)); ok {
			val.VotingPower = int64(value)
			vals = append(vals, val)
		}
	} // sort by address
	vals = hmTypes.SortValidatorByAddress(vals)

	return vals, nil
}

// UpdateLastSpan updates the last span start block
func (k *Keeper) UpdateLastSpan(ctx sdk.Context, id uint64) {
	store := ctx.KVStore(k.storeKey)
	store.Set(LastSpanIDKey, []byte(strconv.FormatUint(id, 10)))
}

// IncrementLastEthBlock increment last eth block
func (k *Keeper) IncrementLastEthBlock(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	lastEthBlock := big.NewInt(0)
	if store.Has(LastProcessedEthBlock) {
		lastEthBlock = lastEthBlock.SetBytes(store.Get(LastProcessedEthBlock))
	}
	store.Set(LastProcessedEthBlock, lastEthBlock.Add(lastEthBlock, big.NewInt(1)).Bytes())
}

// SetLastEthBlock sets last eth block number
func (k *Keeper) SetLastEthBlock(ctx sdk.Context, blockNumber *big.Int) {
	store := ctx.KVStore(k.storeKey)
	store.Set(LastProcessedEthBlock, blockNumber.Bytes())
}

// GetLastEthBlock get last processed Eth block for seed
func (k *Keeper) GetLastEthBlock(ctx sdk.Context) *big.Int {
	store := ctx.KVStore(k.storeKey)
	lastEthBlock := big.NewInt(0)
	if store.Has(LastProcessedEthBlock) {
		lastEthBlock = lastEthBlock.SetBytes(store.Get(LastProcessedEthBlock))
	}
	return lastEthBlock
}

func (k Keeper) GetNextSpanSeed(ctx sdk.Context, contractCaller helper.IContractCaller) (common.Hash, error) {
	lastEthBlock := k.GetLastEthBlock(ctx)

	// increment last processed header block number
	newEthBlock := lastEthBlock.Add(lastEthBlock, big.NewInt(1))
	k.Logger(ctx).Debug("newEthBlock to generate seed", "newEthBlock", newEthBlock)

	// fetch block header from mainchain
	blockHeader, err := contractCaller.GetMainChainBlock(newEthBlock)

	if err != nil {
		k.Logger(ctx).Error("Error fetching block header from mainchain while calculating next span seed", "error", err)
		return common.Hash{}, err
	}

	return blockHeader.Hash(), nil
}

//
// Utils
//

// IterateSpansAndApplyFn interate spans and apply the given function.
func (k *Keeper) IterateSpansAndApplyFn(ctx sdk.Context, f func(span hmTypes.Span) error) error {
	store := ctx.KVStore(k.storeKey)

	// get span iterator
	iterator := sdk.KVStorePrefixIterator(store, SpanPrefixKey)
	defer iterator.Close()

	// loop through spans to get valid spans
	for ; iterator.Valid(); iterator.Next() {
		// unmarshall span
		var result hmTypes.Span
		err := k.cdc.UnmarshalBinaryBare(iterator.Value(), &result)
		if err != nil {
			k.Logger(ctx).Error("Error UnmarshalBinaryBare", "error", err)
			return err
		}
		// call function and return if required
		resultError := f(result)
		if resultError != nil {
			k.Logger(ctx).Error("Error UnmarshalBinaryBare", "error", resultError)
			return resultError
		}
	}
	return nil
}
