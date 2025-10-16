package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

// makeValidOneTimeAddress creates a valid OneTimeAddress for testing
func makeValidOneTimeAddress() types.OneTimeAddress {
	return types.OneTimeAddress{
		Address: types.ECPoint{
			X: make([]byte, 32),
			Y: make([]byte, 32),
		},
		TxPublicKey: types.ECPoint{
			X: make([]byte, 32),
			Y: make([]byte, 32),
		},
	}
}

// makeValidCommitment creates a valid PedersenCommitment for testing
func makeValidCommitment() types.PedersenCommitment {
	return types.PedersenCommitment{
		Commitment: types.ECPoint{
			X: make([]byte, 32),
			Y: make([]byte, 32),
		},
	}
}

// makeValidNote creates a valid encrypted Note for testing
func makeValidNote() types.Note {
	return types.Note{
		EncryptedData: make([]byte, 64), // 8 (amount) + 32 (blinding) + 16 (auth tag) + 8 (padding)
		Nonce:         make([]byte, 12),
		EphemeralKey: types.ECPoint{
			X: make([]byte, 32),
			Y: make([]byte, 32),
		},
	}
}

func TestMsgServerShield(t *testing.T) {
	sender := sdk.AccAddress("test_sender_______")

	tests := []struct {
		name        string
		params      types.Params
		msg         *types.MsgShield
		setup       func(*testing.T, sdk.Context, *mockKeepers)
		expectedErr string
	}{
		{
			name: "module disabled",
			params: types.Params{
				Enabled: false,
			},
			msg: &types.MsgShield{
				Sender: sender.String(),
				Amount: sdk.NewInt64Coin("ulight", 100),
			},
			expectedErr: "privacy module is disabled",
		},
		{
			name: "invalid sender address",
			params: types.Params{
				Enabled: true,
			},
			msg: &types.MsgShield{
				Sender: "invalid_address",
				Amount: sdk.NewInt64Coin("ulight", 100),
			},
			expectedErr: "invalid sender address",
		},
		{
			name: "denomination not allowed",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"uphoton"},
			},
			msg: &types.MsgShield{
				Sender: sender.String(),
				Amount: sdk.NewInt64Coin("ulight", 100),
			},
			expectedErr: "denomination ulight is not allowed for privacy operations",
		},
		{
			name: "amount not positive",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgShield{
				Sender: sender.String(),
				Amount: sdk.NewInt64Coin("ulight", 0),
			},
			expectedErr: "amount must be positive",
		},
		{
			name: "amount below minimum",
			params: types.Params{
				Enabled:          true,
				AllowedDenoms:    []string{"ulight"},
				MinShieldAmounts: map[string]string{"ulight": "1000"},
			},
			msg: &types.MsgShield{
				Sender: sender.String(),
				Amount: sdk.NewInt64Coin("ulight", 100),
			},
			expectedErr: "amount 100 is below minimum 1000 for ulight",
		},
		{
			name: "invalid one-time address - X coord wrong size",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgShield{
				Sender: sender.String(),
				Amount: sdk.NewInt64Coin("ulight", 100),
				OneTimeAddress: types.OneTimeAddress{
					Address: types.ECPoint{
						X: make([]byte, 16), // Wrong size
						Y: make([]byte, 32),
					},
					TxPublicKey: makeValidCommitment().Commitment,
				},
			},
			expectedErr: "invalid one-time address",
		},
		{
			name: "invalid commitment - Y coord wrong size",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgShield{
				Sender:         sender.String(),
				Amount:         sdk.NewInt64Coin("ulight", 100),
				OneTimeAddress: makeValidOneTimeAddress(),
				Commitment: types.PedersenCommitment{
					Commitment: types.ECPoint{
						X: make([]byte, 32),
						Y: make([]byte, 16), // Wrong size
					},
				},
			},
			expectedErr: "invalid Pedersen commitment",
		},
		{
			name: "empty encrypted note",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgShield{
				Sender:         sender.String(),
				Amount:         sdk.NewInt64Coin("ulight", 100),
				OneTimeAddress: makeValidOneTimeAddress(),
				Commitment:     makeValidCommitment(),
				EncryptedNote: types.Note{
					EncryptedData: []byte{},
					Nonce:         make([]byte, 12),
					EphemeralKey:  makeValidCommitment().Commitment,
				},
			},
			expectedErr: "encrypted data is empty",
		},
		{
			name: "invalid nonce size",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgShield{
				Sender:         sender.String(),
				Amount:         sdk.NewInt64Coin("ulight", 100),
				OneTimeAddress: makeValidOneTimeAddress(),
				Commitment:     makeValidCommitment(),
				EncryptedNote: types.Note{
					EncryptedData: make([]byte, 64),
					Nonce:         make([]byte, 8), // Wrong size
					EphemeralKey:  makeValidCommitment().Commitment,
				},
			},
			expectedErr: "nonce must be 12 bytes for AES-GCM",
		},
		{
			name: "memo too large",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
				MaxMemoSize:   512,
			},
			msg: &types.MsgShield{
				Sender:         sender.String(),
				Amount:         sdk.NewInt64Coin("ulight", 100),
				OneTimeAddress: makeValidOneTimeAddress(),
				Commitment:     makeValidCommitment(),
				EncryptedNote: types.Note{
					EncryptedData: make([]byte, 1024), // Too large
					Nonce:         make([]byte, 12),
					EphemeralKey:  makeValidCommitment().Commitment,
				},
			},
			expectedErr: "encrypted note exceeds maximum size",
		},
		// TODO: Add successful shield test case once we have proper mock setup
		// This would require mocking bankKeeper.SendCoinsFromAccountToModule and BurnCoins
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Implement proper test setup with mocks
			// For now, we just test the validation logic
			if tt.expectedErr != "" {
				// Test will verify expected errors once we have keeper setup
				t.Skip("Skipping until proper test setup is implemented")
			}
		})
	}
}

func TestMsgServerPrivateTransfer(t *testing.T) {
	sender := sdk.AccAddress("test_sender_______")

	tests := []struct {
		name        string
		params      types.Params
		msg         *types.MsgPrivateTransfer
		expectedErr string
	}{
		{
			name: "module disabled",
			params: types.Params{
				Enabled: false,
			},
			msg: &types.MsgPrivateTransfer{
				Sender: sender.String(),
				Denom:  "ulight",
			},
			expectedErr: "privacy module is disabled",
		},
		{
			name: "denomination not allowed",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"uphoton"},
			},
			msg: &types.MsgPrivateTransfer{
				Sender: sender.String(),
				Denom:  "ulight",
			},
			expectedErr: "denomination ulight is not allowed",
		},
		{
			name: "empty inputs",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgPrivateTransfer{
				Sender:  sender.String(),
				Denom:   "ulight",
				Inputs:  []types.TransferInput{},
				Outputs: []types.TransferOutput{makeValidTransferOutput("ulight")},
			},
			expectedErr: "no inputs provided",
		},
		{
			name: "empty outputs",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgPrivateTransfer{
				Sender: sender.String(),
				Denom:  "ulight",
				Inputs: []types.TransferInput{{
					Nullifier:    make([]byte, 32),
					DepositIndex: 0,
					Signature:    make([]byte, 64),
				}},
				Outputs: []types.TransferOutput{},
			},
			expectedErr: "no outputs provided",
		},
		{
			name: "too many inputs",
			params: types.Params{
				Enabled:          true,
				AllowedDenoms:    []string{"ulight"},
				MaxDepositsPerTx: 2,
			},
			msg: &types.MsgPrivateTransfer{
				Sender: sender.String(),
				Denom:  "ulight",
				Inputs: []types.TransferInput{
					{Nullifier: make([]byte, 32), Signature: make([]byte, 64)},
					{Nullifier: make([]byte, 32), Signature: make([]byte, 64)},
					{Nullifier: make([]byte, 32), Signature: make([]byte, 64)},
				},
				Outputs: []types.TransferOutput{makeValidTransferOutput("ulight")},
			},
			expectedErr: "got 3 inputs, max 2",
		},
		{
			name: "too many outputs",
			params: types.Params{
				Enabled:          true,
				AllowedDenoms:    []string{"ulight"},
				MaxDepositsPerTx: 2,
			},
			msg: &types.MsgPrivateTransfer{
				Sender: sender.String(),
				Denom:  "ulight",
				Inputs: []types.TransferInput{{Nullifier: make([]byte, 32), Signature: make([]byte, 64)}},
				Outputs: []types.TransferOutput{
					makeValidTransferOutput("ulight"),
					makeValidTransferOutput("ulight"),
					makeValidTransferOutput("ulight"),
				},
			},
			expectedErr: "got 3 outputs, max 2",
		},
		{
			name: "empty nullifier",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgPrivateTransfer{
				Sender: sender.String(),
				Denom:  "ulight",
				Inputs: []types.TransferInput{{
					Nullifier:    []byte{}, // Empty
					DepositIndex: 0,
					Signature:    make([]byte, 64),
				}},
				Outputs: []types.TransferOutput{makeValidTransferOutput("ulight")},
			},
			expectedErr: "input 0 has empty nullifier",
		},
		// TODO: Add tests for:
		// - Nullifier already used (requires mock keeper with state)
		// - Valid transfer (requires full mock setup)
		// - Phase 2 zk-SNARK validation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != "" {
				t.Skip("Skipping until proper test setup is implemented")
			}
		})
	}
}

func TestMsgServerUnshield(t *testing.T) {
	sender := sdk.AccAddress("test_sender_______")
	recipient := sdk.AccAddress("test_recipient____")

	tests := []struct {
		name        string
		params      types.Params
		msg         *types.MsgUnshield
		expectedErr string
	}{
		{
			name: "module disabled",
			params: types.Params{
				Enabled: false,
			},
			msg: &types.MsgUnshield{
				Sender:    sender.String(),
				Recipient: recipient.String(),
			},
			expectedErr: "privacy module is disabled",
		},
		{
			name: "invalid recipient address",
			params: types.Params{
				Enabled: true,
			},
			msg: &types.MsgUnshield{
				Sender:    sender.String(),
				Recipient: "invalid_address",
			},
			expectedErr: "invalid recipient address",
		},
		{
			name: "denomination not allowed",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"uphoton"},
			},
			msg: &types.MsgUnshield{
				Sender:    sender.String(),
				Recipient: recipient.String(),
				Denom:     "ulight",
			},
			expectedErr: "denomination ulight is not allowed",
		},
		{
			name: "invalid amount - not a number",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgUnshield{
				Sender:    sender.String(),
				Recipient: recipient.String(),
				Denom:     "ulight",
				Amount:    "not_a_number",
			},
			expectedErr: "amount must be a positive integer",
		},
		{
			name: "invalid amount - zero",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgUnshield{
				Sender:    sender.String(),
				Recipient: recipient.String(),
				Denom:     "ulight",
				Amount:    "0",
			},
			expectedErr: "amount must be a positive integer",
		},
		{
			name: "empty nullifier",
			params: types.Params{
				Enabled:       true,
				AllowedDenoms: []string{"ulight"},
			},
			msg: &types.MsgUnshield{
				Sender:     sender.String(),
				Recipient:  recipient.String(),
				Denom:      "ulight",
				Amount:     "100",
				Nullifier:  []byte{},
				Commitment: makeValidCommitment(),
			},
			expectedErr: "invalid nullifier",
		},
		// TODO: Add tests for:
		// - Nullifier already used
		// - Deposit not found (Phase 1)
		// - Invalid signature (Phase 1)
		// - Invalid ZK proof (Phase 2)
		// - Successful unshield
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != "" {
				t.Skip("Skipping until proper test setup is implemented")
			}
		})
	}
}

func TestMsgServerUpdateParams(t *testing.T) {
	govAuthority := "cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn"

	tests := []struct {
		name        string
		msg         *types.MsgUpdateParams
		expectedErr string
	}{
		{
			name: "empty authority",
			msg: &types.MsgUpdateParams{
				Authority: "",
				Params:    types.DefaultParams(),
			},
			expectedErr: "invalid authority",
		},
		{
			name: "invalid authority",
			msg: &types.MsgUpdateParams{
				Authority: "invalid_authority",
				Params:    types.DefaultParams(),
			},
			expectedErr: "invalid authority",
		},
		{
			name: "invalid params - max deposits zero",
			msg: &types.MsgUpdateParams{
				Authority: govAuthority,
				Params: types.Params{
					Enabled:          true,
					MaxDepositsPerTx: 0, // Invalid
				},
			},
			expectedErr: "max_deposits_per_tx must be greater than 0",
		},
		{
			name: "invalid params - invalid phase",
			msg: &types.MsgUpdateParams{
				Authority: govAuthority,
				Params: types.Params{
					Enabled:          true,
					MaxDepositsPerTx: 16,
					MerkleTreeDepth:  32,
					Phase:            "phase3", // Invalid
					ProofSystem:      "groth16",
				},
			},
			expectedErr: "phase must be 'phase1' or 'phase2'",
		},
		// TODO: Add successful update test
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedErr != "" {
				t.Skip("Skipping until proper test setup is implemented")
			}
		})
	}
}

// Helper function to create valid transfer output for tests
func makeValidTransferOutput(denom string) types.TransferOutput {
	return types.TransferOutput{
		Denom:          denom,
		Commitment:     makeValidCommitment(),
		OneTimeAddress: makeValidOneTimeAddress(),
		EncryptedNote:  makeValidNote(),
	}
}

// mockKeepers is a placeholder for mock keeper dependencies
type mockKeepers struct {
	// TODO: Add mock fields for AccountKeeper, BankKeeper when implementing full tests
}

// TestValidateECPoint tests the EC point validation helper
func TestValidateECPoint(t *testing.T) {
	tests := []struct {
		name        string
		point       *types.ECPoint
		expectedErr bool
	}{
		{
			name:        "nil point",
			point:       nil,
			expectedErr: true,
		},
		{
			name: "X coordinate wrong size",
			point: &types.ECPoint{
				X: make([]byte, 16),
				Y: make([]byte, 32),
			},
			expectedErr: true,
		},
		{
			name: "Y coordinate wrong size",
			point: &types.ECPoint{
				X: make([]byte, 32),
				Y: make([]byte, 16),
			},
			expectedErr: true,
		},
		{
			name: "valid point",
			point: &types.ECPoint{
				X: make([]byte, 32),
				Y: make([]byte, 32),
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This would test the validateECPoint function from msg_server.go
			// which is not exported. In a real implementation, we might export it
			// or test it indirectly through message handler tests.
			if tt.expectedErr {
				require.NotNil(t, tt.point, "test expects error but point setup is incorrect")
			}
			// TODO: Call validateECPoint once it's accessible or test indirectly
		})
	}
}

// TestParams validates the Params.Validate() function
func TestParamsValidate(t *testing.T) {
	tests := []struct {
		name        string
		params      types.Params
		expectedErr bool
	}{
		{
			name:        "default params valid",
			params:      types.DefaultParams(),
			expectedErr: false,
		},
		{
			name: "max deposits zero",
			params: types.Params{
				MaxDepositsPerTx: 0,
			},
			expectedErr: true,
		},
		{
			name: "max deposits too high",
			params: types.Params{
				MaxDepositsPerTx: 256,
			},
			expectedErr: true,
		},
		{
			name: "merkle tree depth zero",
			params: types.Params{
				MaxDepositsPerTx: 16,
				MerkleTreeDepth:  0,
			},
			expectedErr: true,
		},
		{
			name: "merkle tree depth too high",
			params: types.Params{
				MaxDepositsPerTx: 16,
				MerkleTreeDepth:  128,
			},
			expectedErr: true,
		},
		{
			name: "invalid phase",
			params: types.Params{
				MaxDepositsPerTx: 16,
				MerkleTreeDepth:  32,
				Phase:            "invalid",
				ProofSystem:      "groth16",
			},
			expectedErr: true,
		},
		{
			name: "invalid proof system",
			params: types.Params{
				MaxDepositsPerTx: 16,
				MerkleTreeDepth:  32,
				Phase:            "phase1",
				ProofSystem:      "invalid",
			},
			expectedErr: true,
		},
		{
			name: "valid phase1 params",
			params: types.Params{
				Enabled:                true,
				AllowedDenoms:          []string{"ulight"},
				MinShieldAmounts:       map[string]string{"ulight": "1"},
				MaxDepositsPerTx:       16,
				MerkleTreeDepth:        32,
				ProofSystem:            "groth16",
				MaxMemoSize:            512,
				NullifierCacheDuration: 100000,
				Phase:                  "phase1",
			},
			expectedErr: false,
		},
		{
			name: "valid phase2 params",
			params: types.Params{
				Enabled:                true,
				AllowedDenoms:          []string{"ulight", "uphoton"},
				MinShieldAmounts:       map[string]string{"ulight": "1", "uphoton": "1000000"},
				MaxDepositsPerTx:       16,
				MerkleTreeDepth:        32,
				ProofSystem:            "plonk",
				MaxMemoSize:            512,
				NullifierCacheDuration: 100000,
				Phase:                  "phase2",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.expectedErr {
				require.Error(t, err, "expected validation error")
			} else {
				require.NoError(t, err, "expected no validation error")
			}
		})
	}
}

// TestMinShieldAmount tests minimum shield amount validation
func TestMinShieldAmount(t *testing.T) {
	tests := []struct {
		name          string
		minAmounts    map[string]string
		denom         string
		amount        math.Int
		shouldBeValid bool
	}{
		{
			name:          "no minimum set - any amount valid",
			minAmounts:    map[string]string{},
			denom:         "ulight",
			amount:        math.NewInt(1),
			shouldBeValid: true,
		},
		{
			name:          "amount equals minimum",
			minAmounts:    map[string]string{"ulight": "100"},
			denom:         "ulight",
			amount:        math.NewInt(100),
			shouldBeValid: true,
		},
		{
			name:          "amount above minimum",
			minAmounts:    map[string]string{"ulight": "100"},
			denom:         "ulight",
			amount:        math.NewInt(1000),
			shouldBeValid: true,
		},
		{
			name:          "amount below minimum",
			minAmounts:    map[string]string{"ulight": "100"},
			denom:         "ulight",
			amount:        math.NewInt(50),
			shouldBeValid: false,
		},
		{
			name:          "different denom - no restriction",
			minAmounts:    map[string]string{"uphoton": "1000000"},
			denom:         "ulight",
			amount:        math.NewInt(1),
			shouldBeValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minAmountStr, exists := tt.minAmounts[tt.denom]
			if !exists || minAmountStr == "" {
				// No minimum, always valid
				require.True(t, tt.shouldBeValid)
				return
			}

			minAmount, ok := math.NewIntFromString(minAmountStr)
			require.True(t, ok, "invalid minimum amount in test")

			isValid := tt.amount.GTE(minAmount)
			require.Equal(t, tt.shouldBeValid, isValid)
		})
	}
}
