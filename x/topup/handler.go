package topup

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/maticnetwork/heimdall/helper"
	"github.com/maticnetwork/heimdall/x/topup/keeper"
	"github.com/maticnetwork/heimdall/x/topup/types"
)

// NewHandler ...
func NewHandler(k keeper.Keeper, contractCaller helper.IContractCaller) sdk.Handler {
	msgServer := keeper.NewMsgServerImpl(k, contractCaller)
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case *types.MsgTopup:
			res, err := msgServer.Topup(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgWithdrawFee:
			res, err := msgServer.WithdrawFee(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)

		// this line is used by starport scaffolding # 1
		default:
			errMsg := fmt.Sprintf("unrecognized %s message type: %T", types.ModuleName, msg)
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}
