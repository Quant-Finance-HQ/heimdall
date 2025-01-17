package keeper_test

import (
	"math/big"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/maticnetwork/heimdall/helper/mocks"

	"github.com/maticnetwork/heimdall/x/topup/test_helper"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/maticnetwork/heimdall/app"
	hmTypes "github.com/maticnetwork/heimdall/types"
	"github.com/maticnetwork/heimdall/types/simulation"
	checkpointTypes "github.com/maticnetwork/heimdall/x/checkpoint/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KeeperTestSuite struct {
	suite.Suite

	contractCaller mocks.IContractCaller
	app            *app.HeimdallApp
	ctx            sdk.Context
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.app, suite.ctx, _ = test_helper.CreateTestApp(false)
	suite.contractCaller = mocks.IContractCaller{}
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

// Tests

func (suite *KeeperTestSuite) TestTopupSequenceSet() {
	t, initApp, ctx := suite.T(), suite.app, suite.ctx
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	topupSequence := strconv.Itoa(simulation.RandIntBetween(r1, 1000, 100000))

	initApp.TopupKeeper.SetTopupSequence(ctx, topupSequence)

	actualResult := initApp.TopupKeeper.HasTopupSequence(ctx, topupSequence)
	require.Equal(t, true, actualResult)
}

// tests setter/getters for Dividend account
func (suite *KeeperTestSuite) TestDividendAccount() {
	t, initApp, ctx := suite.T(), suite.app, suite.ctx

	generatedAddress, _ := sdk.AccAddressFromHex("some-address")
	dividendAccount := hmTypes.DividendAccount{
		User:      generatedAddress.String(),
		FeeAmount: big.NewInt(0).String(),
	}
	err := initApp.TopupKeeper.AddDividendAccount(ctx, dividendAccount)
	require.Nil(t, err)
	ok := initApp.TopupKeeper.CheckIfDividendAccountExists(ctx, []byte(dividendAccount.User))
	require.Equal(t, ok, true)
}

func (suite *KeeperTestSuite) TestAddFeeToDividendAccount() {
	t, initApp, ctx := suite.T(), suite.app, suite.ctx
	address := sdk.AccAddress("234452")
	amount, ok := big.NewInt(0).SetString("0", 10)
	require.True(t, ok)
	err := initApp.TopupKeeper.AddFeeToDividendAccount(ctx, address, amount)
	require.NoError(t, err)
	dividendAccount, err := initApp.TopupKeeper.GetDividendAccountByAddress(ctx, address)
	require.NoError(t, err)
	require.NotNil(t, dividendAccount)
	actualResult, ok := big.NewInt(0).SetString(dividendAccount.FeeAmount, 10)
	require.True(t, ok)
	require.NoError(t, err)
	require.Equal(t, amount, actualResult)
}

func (suite *KeeperTestSuite) TestDividendAccountTree() {
	t := suite.T()

	divAccounts := make([]*hmTypes.DividendAccount, 5)
	for i := 0; i < len(divAccounts); i++ {
		newDivAcc := hmTypes.NewDividendAccount(
			sdk.AccAddress("1234"),
			big.NewInt(0).String(),
		)
		divAccounts[i] = &newDivAcc
	}

	accountRoot, err := checkpointTypes.GetAccountRootHash(divAccounts)
	require.NotNil(t, accountRoot)
	require.NoError(t, err)

	accountProof, _, err := checkpointTypes.GetAccountProof(divAccounts, sdk.AccAddress("1234"))
	require.NotNil(t, accountProof)
	require.NoError(t, err)

	leafHash, err := divAccounts[0].CalculateHash()
	require.NotNil(t, leafHash)
	require.NoError(t, err)
}
