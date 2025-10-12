package e2e

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *IntegrationTestSuite) testBankTokenTransfer() {
	s.Run("send tokens between accounts", func() {
		var (
			valIdx        = 0
			c             = s.chainA
			chainEndpoint = fmt.Sprintf("http://%s", s.valResources[c.id][valIdx].GetHostPort("1317/tcp"))
		)

		// define one sender and two recipient accounts
		alice, _ := c.genesisAccounts[1].keyInfo.GetAddress()
		bob, _ := c.genesisAccounts[2].keyInfo.GetAddress()
		charlie, _ := c.genesisAccounts[3].keyInfo.GetAddress()

		var beforeAliceULBalance,
			beforeBobULBalance,
			beforeCharlieULBalance,
			afterAliceULBalance,
			afterBobULBalance,
			afterCharlieULBalance sdk.Coin

		// get balances of sender and recipient accounts
		s.Require().Eventually(
			func() bool {
				beforeAliceULBalance = s.queryBalance(chainEndpoint, alice.String(), ulDenom)
				beforeBobULBalance = s.queryBalance(chainEndpoint, bob.String(), ulDenom)
				beforeCharlieULBalance = s.queryBalance(chainEndpoint, charlie.String(), ulDenom)

				return beforeAliceULBalance.IsValid() && beforeBobULBalance.IsValid() && beforeCharlieULBalance.IsValid()
			},
			10*time.Second,
			time.Second,
		)

		// alice sends tokens to bob
		s.execBankSend(s.chainA, valIdx, alice.String(), bob.String(), tokenAmount.String(), false)

		// check that the transfer was successful
		s.Require().Eventually(
			func() bool {
				afterAliceULBalance = s.queryBalance(chainEndpoint, alice.String(), ulDenom)
				afterBobULBalance = s.queryBalance(chainEndpoint, bob.String(), ulDenom)

				decremented := beforeAliceULBalance.Sub(tokenAmount).IsEqual(afterAliceULBalance)
				incremented := beforeBobULBalance.Add(tokenAmount).IsEqual(afterBobULBalance)

				return decremented && incremented
			},
			10*time.Second,
			time.Second,
		)

		// save the updated account balances of alice and bob
		beforeAliceULBalance, beforeBobULBalance = afterAliceULBalance, afterBobULBalance

		// alice sends tokens to bob and charlie, at once
		s.execBankMultiSend(s.chainA, valIdx, alice.String(),
			[]string{bob.String(), charlie.String()}, tokenAmount.String(), false)

		s.Require().Eventually(
			func() bool {
				afterAliceULBalance = s.queryBalance(chainEndpoint, alice.String(), ulDenom)
				afterBobULBalance = s.queryBalance(chainEndpoint, bob.String(), ulDenom)
				afterCharlieULBalance = s.queryBalance(chainEndpoint, charlie.String(), ulDenom)

				// assert alice's account gets decremented the amount of tokens twice
				decremented := beforeAliceULBalance.Sub(tokenAmount).Sub(tokenAmount).IsEqual(afterAliceULBalance)
				incremented := beforeBobULBalance.Add(tokenAmount).IsEqual(afterBobULBalance) &&
					beforeCharlieULBalance.Add(tokenAmount).IsEqual(afterCharlieULBalance)

				return decremented && incremented
			},
			10*time.Second,
			time.Second,
		)
	})

	s.Run("send tokens with l fees", func() {
		var (
			valIdx = 0
			c      = s.chainA
		)
		alice, _ := c.genesisAccounts[1].keyInfo.GetAddress()
		bob, _ := c.genesisAccounts[2].keyInfo.GetAddress()

		// alice sends tokens to bob should fail because doesn't use photons for the fees.
		lFees := sdk.NewCoin(ulDenom, standardFees.Amount)
		s.execBankSend(s.chainA, valIdx, alice.String(), bob.String(),
			tokenAmount.String(), true, withKeyValue(flagFees, lFees))
	})
}
