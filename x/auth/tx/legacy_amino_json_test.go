package tx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/x/auth/migrations/legacytx"
	"cosmossdk.io/x/auth/signing"
	"cosmossdk.io/x/auth/types"
	txsigning "cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/tx/signing/aminojson"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

var (
	_, pubkey1, addr1 = testdata.KeyTestPubAddr()
	_, _, addr2       = testdata.KeyTestPubAddr()

	coins   = sdk.Coins{sdk.NewInt64Coin("foocoin", 10)}
	gas     = uint64(10000)
	msg     = &types.MsgUpdateParams{Authority: addr1.String()}
	memo    = "foo"
	timeout = uint64(10)
)

func buildTx(t *testing.T, bldr *builder) {
	t.Helper()
	bldr.SetFeeAmount(coins)
	bldr.SetGasLimit(gas)
	bldr.SetMemo(memo)
	bldr.SetTimeoutHeight(timeout)
	require.NoError(t, bldr.SetMsgs(msg))
}

func TestLegacyAminoJSONHandler_GetSignBytes(t *testing.T) {
	legacytx.RegressionTestingAminoCodec = codec.NewLegacyAmino()
	var (
		chainID        = "test-chain"
		accNum  uint64 = 7
		seqNum  uint64 = 7
	)

	testcases := []struct {
		name           string
		signer         string
		malleate       func(builder2 *builder)
		expectedSignBz []byte
	}{
		{
			"signer which is also fee payer (no tips)", addr1.String(),
			func(w *builder) {},
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas}, []sdk.Msg{msg}, memo),
		},

		{
			"explicit fee payer", addr1.String(),
			func(w *builder) { w.SetFeePayer(addr2) },
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas, Payer: addr2.String()}, []sdk.Msg{msg}, memo),
		},
		{
			"explicit fee granter", addr1.String(),
			func(w *builder) { w.SetFeeGranter(addr2) },
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas, Granter: addr2.String()}, []sdk.Msg{msg}, memo),
		},
		{
			"explicit fee payer and fee granter", addr1.String(),
			func(w *builder) {
				w.SetFeePayer(addr2)
				w.SetFeeGranter(addr2)
			},
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas, Payer: addr2.String(), Granter: addr2.String()}, []sdk.Msg{msg}, memo),
		},
	}

	handler := signModeLegacyAminoJSONHandler{}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bldr := newBuilder(nil)
			buildTx(t, bldr)
			tx := bldr.GetTx()
			tc.malleate(bldr)

			signingData := signing.SignerData{
				Address:       tc.signer,
				ChainID:       chainID,
				AccountNumber: accNum,
				Sequence:      seqNum,
			}
			signBz, err := handler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signingData, tx)
			require.NoError(t, err)

			require.Equal(t, tc.expectedSignBz, signBz)
		})
	}

	bldr := newBuilder(nil)
	buildTx(t, bldr)
	tx := bldr.GetTx()
	signingData := signing.SignerData{
		Address:       addr1.String(),
		ChainID:       chainID,
		AccountNumber: accNum,
		Sequence:      seqNum,
		PubKey:        pubkey1,
	}

	// expect error with wrong sign mode
	_, err := handler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_DIRECT, signingData, tx)
	require.Error(t, err)

	// expect error with extension options
	bldr = newBuilder(nil)
	buildTx(t, bldr)
	any, err := cdctypes.NewAnyWithValue(testdata.NewTestMsg())
	require.NoError(t, err)
	bldr.tx.Body.ExtensionOptions = []*cdctypes.Any{any}
	tx = bldr.GetTx()
	_, err = handler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signingData, tx)
	require.Error(t, err)

	// expect error with non-critical extension options
	bldr = newBuilder(nil)
	buildTx(t, bldr)
	bldr.tx.Body.NonCriticalExtensionOptions = []*cdctypes.Any{any}
	tx = bldr.GetTx()
	_, err = handler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signingData, tx)
	require.Error(t, err)
}

func TestLegacyAminoJSONHandler_AllGetSignBytesComparison(t *testing.T) {
	var (
		chainID = "test-chain"
		accNum  uint64
		seqNum  uint64 = 7
	)

	modeHandler := aminojson.NewSignModeHandler(aminojson.SignModeHandlerOptions{})
	mode, _ := signing.APISignModeToInternal(modeHandler.Mode())
	legacyAmino := codec.NewLegacyAmino()
	legacytx.RegressionTestingAminoCodec = legacyAmino
	legacy.RegisterAminoMsg(legacyAmino, &types.MsgUpdateParams{}, "cosmos-sdk/x/auth/MsgUpdateParams")

	testcases := []struct {
		name           string
		signer         string
		malleate       func(*builder)
		expectedSignBz []byte
	}{
		{
			"signer which is also fee payer (no tips)", addr1.String(),
			func(w *builder) {},
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas}, []sdk.Msg{msg}, memo),
		},

		{
			"explicit fee payer", addr1.String(),
			func(w *builder) { w.SetFeePayer(addr2) },
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas, Payer: addr2.String()}, []sdk.Msg{msg}, memo),
		},
		{
			"explicit fee granter", addr1.String(),
			func(w *builder) { w.SetFeeGranter(addr2) },
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas, Granter: addr2.String()}, []sdk.Msg{msg}, memo),
		},
		{
			"explicit fee payer and fee granter", addr1.String(),
			func(w *builder) {
				w.SetFeePayer(addr2)
				w.SetFeeGranter(addr2)
			},
			legacytx.StdSignBytes(chainID, accNum, seqNum, timeout, legacytx.StdFee{Amount: coins, Gas: gas, Payer: addr2.String(), Granter: addr2.String()}, []sdk.Msg{msg}, memo),
		},
	}

	handler := signModeLegacyAminoJSONHandler{}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bldr := newBuilder(nil)
			buildTx(t, bldr)
			tx := bldr.GetTx()
			tc.malleate(bldr)

			signingData := signing.SignerData{
				Address:       tc.signer,
				ChainID:       chainID,
				AccountNumber: accNum,
				Sequence:      seqNum,
				PubKey:        pubkey1,
			}
			signBz, err := handler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signingData, tx)
			require.NoError(t, err)

			// compare with new signing
			newSignBz, err := signing.GetSignBytesAdapter(context.Background(), txsigning.NewHandlerMap(modeHandler), mode, signingData, tx)
			require.NoError(t, err)

			require.Equal(t, string(tc.expectedSignBz), string(signBz))
			require.Equal(t, string(tc.expectedSignBz), string(newSignBz))
		})
	}

	legacytx.RegressionTestingAminoCodec = nil
}

func TestLegacyAminoJSONHandler_DefaultMode(t *testing.T) {
	handler := signModeLegacyAminoJSONHandler{}
	require.Equal(t, signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, handler.DefaultMode())
}

func TestLegacyAminoJSONHandler_Modes(t *testing.T) {
	handler := signModeLegacyAminoJSONHandler{}
	require.Equal(t, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON}, handler.Modes())
}
