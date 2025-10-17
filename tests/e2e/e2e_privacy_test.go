package e2e

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Hikari-Chain/hikari-chain/x/privacy/crypto"
	"github.com/Hikari-Chain/hikari-chain/x/privacy/types"
)

// testPrivacyShieldAndUnshield tests the basic shield/unshield workflow
func (s *IntegrationTestSuite) testPrivacyShieldAndUnshield() {
	s.Run("privacy_shield_and_unshield", func() {
		c := s.chainA
		chainEndpoint := fmt.Sprintf("http://%s", s.valResources[c.id][0].GetHostPort("1317/tcp"))

		// Generate privacy key pair for recipient
		keyPair, err := crypto.GenerateStealthKeyPair()
		s.Require().NoError(err)

		viewPubKeyHex := hex.EncodeToString(keyPair.ViewPublicKey.Compressed())
		spendPubKeyHex := hex.EncodeToString(keyPair.SpendPublicKey.Compressed())

		// Get test accounts
		alice, err := c.genesisAccounts[1].keyInfo.GetAddress()
		s.Require().NoError(err)

		// Query initial balances
		initialBalance := s.queryBalance(chainEndpoint, alice.String(), ulDenom)
		s.T().Logf("Alice initial balance: %s", initialBalance.String())

		// Shield amount
		shieldAmount := sdk.NewInt64Coin(ulDenom, 100000)
		s.T().Logf("Shielding %s from Alice to privacy pool", shieldAmount.String())

		// Execute shield transaction
		s.execPrivacyShield(c, 0, alice.String(), shieldAmount.String(), viewPubKeyHex, spendPubKeyHex, false)

		// Wait for balance to decrease (shield + fees)
		s.Require().Eventually(
			func() bool {
				newBalance := s.queryBalance(chainEndpoint, alice.String(), ulDenom)
				// Balance should decrease by at least the shielded amount
				return newBalance.Amount.LT(initialBalance.Amount.Sub(shieldAmount.Amount))
			},
			15*time.Second,
			time.Second,
			"Alice's balance should decrease after shield",
		)

		// Query privacy pool stats
		stats := s.queryPrivacyStats(chainEndpoint)
		s.T().Logf("Privacy stats - Total deposits: %d, Active: %d", stats.TotalDeposits, stats.ActiveDeposits)
		s.Require().Greater(stats.TotalDeposits, uint64(0), "Should have at least one deposit")

		// Query deposits for the denomination
		deposits := s.queryPrivacyDeposits(chainEndpoint, ulDenom)
		s.Require().NotEmpty(deposits, "Should have deposits in the pool")
		s.T().Logf("Found %d deposit(s) in the pool", len(deposits))

		// Find the deposit we just created (it should be the last one)
		var ownedDepositIndex uint64
		found := false
		for _, deposit := range deposits {
			// Try to scan this deposit with our keys
			if s.scanDeposit(deposit, keyPair.ViewPrivateKey, keyPair.SpendPublicKey) {
				ownedDepositIndex = deposit.Index
				found = true
				s.T().Logf("Found our deposit at index %d", ownedDepositIndex)
				break
			}
		}
		s.Require().True(found, "Should find our deposit in the pool")

		// Now unshield back to Alice's account
		bob, err := c.genesisAccounts[2].keyInfo.GetAddress()
		s.Require().NoError(err)

		bobInitialBalance := s.queryBalance(chainEndpoint, bob.String(), ulDenom)
		s.T().Logf("Bob initial balance: %s", bobInitialBalance.String())

		unshieldAmount := sdk.NewInt64Coin(ulDenom, 50000)
		s.T().Logf("Unshielding %s to Bob", unshieldAmount.String())

		// Convert private keys to hex
		viewPrivKeyBytes := make([]byte, 32)
		viewPrivKeyB := keyPair.ViewPrivateKey.Bytes()
		copy(viewPrivKeyBytes[32-len(viewPrivKeyB):], viewPrivKeyB)

		spendPrivKeyBytes := make([]byte, 32)
		spendPrivKeyB := keyPair.SpendPrivateKey.Bytes()
		copy(spendPrivKeyBytes[32-len(spendPrivKeyB):], spendPrivKeyB)

		viewPrivKeyHex := hex.EncodeToString(viewPrivKeyBytes)
		spendPrivKeyHex := hex.EncodeToString(spendPrivKeyBytes)

		// Execute unshield transaction
		s.execPrivacyUnshield(c, 0, alice.String(), bob.String(), unshieldAmount.String(), ulDenom,
			ownedDepositIndex, viewPrivKeyHex, spendPrivKeyHex, false)

		// Wait for Bob's balance to increase
		expectedBobBalance := bobInitialBalance.Add(unshieldAmount)
		s.Require().Eventually(
			func() bool {
				newBalance := s.queryBalance(chainEndpoint, bob.String(), ulDenom)
				return newBalance.IsEqual(expectedBobBalance)
			},
			15*time.Second,
			time.Second,
			"Bob's balance should increase after unshield",
		)

		s.T().Logf("Successfully shielded and unshielded tokens")
	})
}

// testPrivacyTransfer tests private-to-private transfers
func (s *IntegrationTestSuite) testPrivacyTransfer() {
	s.Run("privacy_private_transfer", func() {
		c := s.chainA
		chainEndpoint := fmt.Sprintf("http://%s", s.valResources[c.id][0].GetHostPort("1317/tcp"))

		// Generate key pairs for sender and recipient
		senderKeys, err := crypto.GenerateStealthKeyPair()
		s.Require().NoError(err)

		recipientKeys, err := crypto.GenerateStealthKeyPair()
		s.Require().NoError(err)

		// Get Alice's address for funding
		alice, err := c.genesisAccounts[1].keyInfo.GetAddress()
		s.Require().NoError(err)

		// Shield tokens to sender's privacy keys
		shieldAmount := sdk.NewInt64Coin(ulDenom, 200000)
		s.T().Logf("Shielding %s to sender's privacy address", shieldAmount.String())

		senderViewPubKeyHex := hex.EncodeToString(senderKeys.ViewPublicKey.Compressed())
		senderSpendPubKeyHex := hex.EncodeToString(senderKeys.SpendPublicKey.Compressed())

		s.execPrivacyShield(c, 0, alice.String(), shieldAmount.String(),
			senderViewPubKeyHex, senderSpendPubKeyHex, false)

		// Wait a bit for the transaction to be processed
		time.Sleep(3 * time.Second)

		// Find sender's deposit
		deposits := s.queryPrivacyDeposits(chainEndpoint, ulDenom)
		var senderDepositIndex uint64
		found := false
		for _, deposit := range deposits {
			if s.scanDeposit(deposit, senderKeys.ViewPrivateKey, senderKeys.SpendPublicKey) {
				// Check if this deposit is unspent
				if len(deposit.Nullifier) == 0 {
					senderDepositIndex = deposit.Index
					found = true
					s.T().Logf("Found sender's unspent deposit at index %d", senderDepositIndex)
					break
				}
			}
		}
		s.Require().True(found, "Should find sender's unspent deposit")

		// Now perform a private transfer to recipient
		transferAmount := uint64(100000)
		s.T().Logf("Performing private transfer of %d to recipient", transferAmount)

		// Convert sender's private keys to hex
		senderViewPrivKeyBytes := make([]byte, 32)
		senderViewPrivKeyB := senderKeys.ViewPrivateKey.Bytes()
		copy(senderViewPrivKeyBytes[32-len(senderViewPrivKeyB):], senderViewPrivKeyB)

		senderSpendPrivKeyBytes := make([]byte, 32)
		senderSpendPrivKeyB := senderKeys.SpendPrivateKey.Bytes()
		copy(senderSpendPrivKeyBytes[32-len(senderSpendPrivKeyB):], senderSpendPrivKeyB)

		senderViewPrivKeyHex := hex.EncodeToString(senderViewPrivKeyBytes)
		senderSpendPrivKeyHex := hex.EncodeToString(senderSpendPrivKeyBytes)

		// Recipient's public keys
		recipientViewPubKeyHex := hex.EncodeToString(recipientKeys.ViewPublicKey.Compressed())
		recipientSpendPubKeyHex := hex.EncodeToString(recipientKeys.SpendPublicKey.Compressed())

		// Execute private transfer
		// Format: amount,view-pub-key,spend-pub-key
		outputSpec := fmt.Sprintf("%d,%s,%s", transferAmount, recipientViewPubKeyHex, recipientSpendPubKeyHex)
		s.execPrivacyTransfer(c, 0, alice.String(), ulDenom, senderDepositIndex,
			outputSpec, senderViewPrivKeyHex, senderSpendPrivKeyHex, false)

		// Wait for the transfer to be processed
		time.Sleep(3 * time.Second)

		// Verify recipient can find their deposit
		deposits = s.queryPrivacyDeposits(chainEndpoint, ulDenom)
		recipientFound := false
		for _, deposit := range deposits {
			if s.scanDeposit(deposit, recipientKeys.ViewPrivateKey, recipientKeys.SpendPublicKey) {
				if len(deposit.Nullifier) == 0 {
					recipientFound = true
					s.T().Logf("Recipient found their deposit at index %d", deposit.Index)
					break
				}
			}
		}
		s.Require().True(recipientFound, "Recipient should find their deposit")

		// Verify sender's deposit is now spent (nullifier is set)
		senderDeposit := s.queryPrivacyDeposit(chainEndpoint, ulDenom, senderDepositIndex)
		s.Require().NotEmpty(senderDeposit.Nullifier, "Sender's deposit should be spent")

		s.T().Logf("Successfully completed private-to-private transfer")
	})
}

// testPrivacyMultiOutputTransfer tests transfers with multiple outputs (1→N)
func (s *IntegrationTestSuite) testPrivacyMultiOutputTransfer() {
	s.Run("privacy_multi_output_transfer", func() {
		c := s.chainA
		chainEndpoint := fmt.Sprintf("http://%s", s.valResources[c.id][0].GetHostPort("1317/tcp"))

		// Generate key pairs
		senderKeys, err := crypto.GenerateStealthKeyPair()
		s.Require().NoError(err)

		recipient1Keys, err := crypto.GenerateStealthKeyPair()
		s.Require().NoError(err)

		recipient2Keys, err := crypto.GenerateStealthKeyPair()
		s.Require().NoError(err)

		// Get Alice for funding
		alice, err := c.genesisAccounts[1].keyInfo.GetAddress()
		s.Require().NoError(err)

		// Shield tokens to sender
		shieldAmount := sdk.NewInt64Coin(ulDenom, 300000)
		s.T().Logf("Shielding %s for multi-output test", shieldAmount.String())

		senderViewPubKeyHex := hex.EncodeToString(senderKeys.ViewPublicKey.Compressed())
		senderSpendPubKeyHex := hex.EncodeToString(senderKeys.SpendPublicKey.Compressed())

		s.execPrivacyShield(c, 0, alice.String(), shieldAmount.String(),
			senderViewPubKeyHex, senderSpendPubKeyHex, false)

		time.Sleep(3 * time.Second)

		// Find sender's deposit
		deposits := s.queryPrivacyDeposits(chainEndpoint, ulDenom)
		var senderDepositIndex uint64
		found := false
		for _, deposit := range deposits {
			if s.scanDeposit(deposit, senderKeys.ViewPrivateKey, senderKeys.SpendPublicKey) {
				if len(deposit.Nullifier) == 0 {
					senderDepositIndex = deposit.Index
					found = true
					break
				}
			}
		}
		s.Require().True(found, "Should find sender's deposit")

		// Prepare multi-output transfer (split into 2 recipients)
		amount1 := uint64(150000)
		amount2 := uint64(150000)

		recipient1ViewPubKeyHex := hex.EncodeToString(recipient1Keys.ViewPublicKey.Compressed())
		recipient1SpendPubKeyHex := hex.EncodeToString(recipient1Keys.SpendPublicKey.Compressed())

		recipient2ViewPubKeyHex := hex.EncodeToString(recipient2Keys.ViewPublicKey.Compressed())
		recipient2SpendPubKeyHex := hex.EncodeToString(recipient2Keys.SpendPublicKey.Compressed())

		// Convert sender's private keys
		senderViewPrivKeyBytes := make([]byte, 32)
		senderViewPrivKeyB := senderKeys.ViewPrivateKey.Bytes()
		copy(senderViewPrivKeyBytes[32-len(senderViewPrivKeyB):], senderViewPrivKeyB)

		senderSpendPrivKeyBytes := make([]byte, 32)
		senderSpendPrivKeyB := senderKeys.SpendPrivateKey.Bytes()
		copy(senderSpendPrivKeyBytes[32-len(senderSpendPrivKeyB):], senderSpendPrivKeyB)

		senderViewPrivKeyHex := hex.EncodeToString(senderViewPrivKeyBytes)
		senderSpendPrivKeyHex := hex.EncodeToString(senderSpendPrivKeyBytes)

		// Execute transfer with 2 outputs
		// Format: "amount1,view1,spend1 amount2,view2,spend2"
		outputSpecs := fmt.Sprintf("%d,%s,%s %d,%s,%s",
			amount1, recipient1ViewPubKeyHex, recipient1SpendPubKeyHex,
			amount2, recipient2ViewPubKeyHex, recipient2SpendPubKeyHex)

		s.T().Logf("Executing 1→2 transfer: %d + %d", amount1, amount2)
		s.execPrivacyTransfer(c, 0, alice.String(), ulDenom, senderDepositIndex,
			outputSpecs, senderViewPrivKeyHex, senderSpendPrivKeyHex, false)

		time.Sleep(3 * time.Second)

		// Verify both recipients received their deposits
		deposits = s.queryPrivacyDeposits(chainEndpoint, ulDenom)
		recipient1Found := false
		recipient2Found := false

		for _, deposit := range deposits {
			if len(deposit.Nullifier) == 0 {
				if s.scanDeposit(deposit, recipient1Keys.ViewPrivateKey, recipient1Keys.SpendPublicKey) {
					recipient1Found = true
					s.T().Logf("Recipient 1 found deposit at index %d", deposit.Index)
				}
				if s.scanDeposit(deposit, recipient2Keys.ViewPrivateKey, recipient2Keys.SpendPublicKey) {
					recipient2Found = true
					s.T().Logf("Recipient 2 found deposit at index %d", deposit.Index)
				}
			}
		}

		s.Require().True(recipient1Found, "Recipient 1 should find their deposit")
		s.Require().True(recipient2Found, "Recipient 2 should find their deposit")

		s.T().Logf("Successfully completed 1→2 multi-output transfer")
	})
}

// testPrivacyParams tests querying privacy module parameters
func (s *IntegrationTestSuite) testPrivacyParams() {
	s.Run("privacy_params", func() {
		c := s.chainA
		chainEndpoint := fmt.Sprintf("http://%s", s.valResources[c.id][0].GetHostPort("1317/tcp"))

		params := s.queryPrivacyParams(chainEndpoint)
		s.Require().NotNil(params)
		s.T().Logf("Privacy params - Phase: %s, Enabled: %t, Allowed denoms: %v",
			params.Phase, params.Enabled, params.AllowedDenoms)

		s.Require().Equal("phase1", params.Phase, "Should be in phase1")
		s.Require().True(params.Enabled, "Privacy module should be enabled")
		s.Require().NotEmpty(params.AllowedDenoms, "Should have allowed denominations")
	})
}

// Helper functions for privacy module operations

func (s *IntegrationTestSuite) execPrivacyShield(c *chain, valIdx int, from, amount, viewPubKey, spendPubKey string, expectErr bool, opt ...flagOption) {
	opt = append(opt, withKeyValue(flagFrom, from))
	opts := applyOptions(c.id, opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	hikaridCommand := []string{
		hikaridBinary,
		txCommand,
		types.ModuleName,
		"shield",
		amount,
		viewPubKey,
		spendPubKey,
		"-y",
	}

	for flag, value := range opts {
		hikaridCommand = append(hikaridCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.T().Logf("Executing shield command: %s", strings.Join(hikaridCommand, " "))
	s.executeAtomoneTxCommand(ctx, c, hikaridCommand, valIdx, s.expectErrExecValidation(c, valIdx, expectErr))
	s.T().Log("Shield transaction completed")
}

func (s *IntegrationTestSuite) execPrivacyUnshield(c *chain, valIdx int, from, recipient, amount, denom string, depositIndex uint64, viewPrivKey, spendPrivKey string, expectErr bool, opt ...flagOption) {
	opt = append(opt, withKeyValue(flagFrom, from))
	opts := applyOptions(c.id, opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	hikaridCommand := []string{
		hikaridBinary,
		txCommand,
		types.ModuleName,
		"unshield",
		recipient,
		denom,
		amount,
		fmt.Sprintf("%d", depositIndex),
		fmt.Sprintf("--view-key=%s", viewPrivKey),
		fmt.Sprintf("--spend-key=%s", spendPrivKey),
		"-y",
	}

	for flag, value := range opts {
		hikaridCommand = append(hikaridCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.T().Logf("Executing unshield command")
	s.executeAtomoneTxCommand(ctx, c, hikaridCommand, valIdx, s.expectErrExecValidation(c, valIdx, expectErr))
	s.T().Log("Unshield transaction completed")
}

func (s *IntegrationTestSuite) execPrivacyTransfer(c *chain, valIdx int, from, denom string, depositIndex uint64, outputs, viewPrivKey, spendPrivKey string, expectErr bool, opt ...flagOption) {
	opt = append(opt, withKeyValue(flagFrom, from))
	opts := applyOptions(c.id, opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	hikaridCommand := []string{
		hikaridBinary,
		txCommand,
		types.ModuleName,
		"transfer",
		denom,
		fmt.Sprintf("%d", depositIndex),
		outputs,
		fmt.Sprintf("--view-key=%s", viewPrivKey),
		fmt.Sprintf("--spend-key=%s", spendPrivKey),
		"-y",
	}

	for flag, value := range opts {
		hikaridCommand = append(hikaridCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.T().Logf("Executing private transfer command")
	s.executeAtomoneTxCommand(ctx, c, hikaridCommand, valIdx, s.expectErrExecValidation(c, valIdx, expectErr))
	s.T().Log("Private transfer transaction completed")
}

func (s *IntegrationTestSuite) queryPrivacyParams(endpoint string) *types.Params {
	path := fmt.Sprintf("%s/hikari/privacy/v1/params", endpoint)
	body, err := httpGet(path)
	s.Require().NoError(err)

	var res types.QueryParamsResponse
	err = s.cdc.UnmarshalJSON(body, &res)
	s.Require().NoError(err)

	return &res.Params
}

func (s *IntegrationTestSuite) queryPrivacyStats(endpoint string) *types.QueryStatsResponse {
	path := fmt.Sprintf("%s/hikari/privacy/v1/stats", endpoint)
	body, err := httpGet(path)
	s.Require().NoError(err)

	var res types.QueryStatsResponse
	err = s.cdc.UnmarshalJSON(body, &res)
	s.Require().NoError(err)

	return &res
}

func (s *IntegrationTestSuite) queryPrivacyDeposits(endpoint, denom string) []types.PrivateDeposit {
	path := fmt.Sprintf("%s/hikari/privacy/v1/deposits/%s", endpoint, denom)
	body, err := httpGet(path)
	s.Require().NoError(err)

	var res types.QueryDepositsResponse
	err = s.cdc.UnmarshalJSON(body, &res)
	s.Require().NoError(err)

	return res.Deposits
}

func (s *IntegrationTestSuite) queryPrivacyDeposit(endpoint, denom string, index uint64) types.PrivateDeposit {
	path := fmt.Sprintf("%s/hikari/privacy/v1/deposit/%s/%d", endpoint, denom, index)
	body, err := httpGet(path)
	s.Require().NoError(err)

	var res types.QueryDepositResponse
	err = s.cdc.UnmarshalJSON(body, &res)
	s.Require().NoError(err)

	return res.Deposit
}

// scanDeposit attempts to scan a deposit and returns true if it belongs to the given keys
func (s *IntegrationTestSuite) scanDeposit(deposit types.PrivateDeposit, viewPrivKey *big.Int, spendPubKey *crypto.ECPoint) bool {
	// This is a simplified check - in real CLI, we'd decrypt the note and verify ownership
	// For e2e tests, we'll check if we can derive the shared secret

	// Check if one-time address data is present
	if len(deposit.OneTimeAddress.Address.X) == 0 || len(deposit.OneTimeAddress.TxPublicKey.X) == 0 {
		return false
	}

	// Convert protobuf points to crypto points
	txPubKeyX := new(big.Int).SetBytes(deposit.OneTimeAddress.TxPublicKey.X)
	txPubKeyY := new(big.Int).SetBytes(deposit.OneTimeAddress.TxPublicKey.Y)
	txPubKey := crypto.NewECPoint(txPubKeyX, txPubKeyY)

	oneTimeAddrX := new(big.Int).SetBytes(deposit.OneTimeAddress.Address.X)
	oneTimeAddrY := new(big.Int).SetBytes(deposit.OneTimeAddress.Address.Y)
	oneTimeAddr := crypto.NewECPoint(oneTimeAddrX, oneTimeAddrY)

	// Compute shared secret: viewPrivKey * txPubKey
	temp := crypto.ScalarMult(viewPrivKey, txPubKey)
	if temp == nil {
		return false
	}

	sharedSecret := crypto.Hash256(temp.Bytes())
	hs := crypto.HashToScalar(sharedSecret)

	// Derive expected one-time address: Hash(sharedSecret)*G + spendPubKey
	hsG := crypto.ScalarBaseMult(hs)
	expectedAddr := crypto.PointAdd(hsG, spendPubKey)

	if expectedAddr == nil {
		return false
	}

	// Check if the expected address matches the deposit's address
	return expectedAddr.X.Cmp(oneTimeAddr.X) == 0 && expectedAddr.Y.Cmp(oneTimeAddr.Y) == 0
}
