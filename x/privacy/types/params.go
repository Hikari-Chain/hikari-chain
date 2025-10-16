package types

import (
	"fmt"
)

// DefaultParams returns default privacy parameters
func DefaultParams() Params {
	return Params{
		Enabled:                   false, // Start disabled, enable via governance
		AllowedDenoms:             []string{},
		MinShieldAmounts:          make(map[string]string),
		MaxDepositsPerTx:          16,
		MerkleTreeDepth:           32,
		ProofSystem:               "groth16",
		MaxMemoSize:               512,
		NullifierCacheDuration:    100000,
		Phase:                     "phase1",
		ShieldGasCost:             50000,
		UnshieldGasCost:           50000,
		PrivateTransferGasCost:    100000,
		VerifyProofGasCost:        500000,
	}
}

// Validate validates params
func (p Params) Validate() error {
	if p.MaxDepositsPerTx == 0 {
		return fmt.Errorf("max_deposits_per_tx must be greater than 0")
	}
	if p.MaxDepositsPerTx > 128 {
		return fmt.Errorf("max_deposits_per_tx cannot exceed 128")
	}
	if p.MerkleTreeDepth == 0 || p.MerkleTreeDepth > 64 {
		return fmt.Errorf("merkle_tree_depth must be between 1 and 64")
	}
	if p.Phase != "phase1" && p.Phase != "phase2" {
		return fmt.Errorf("phase must be 'phase1' or 'phase2'")
	}
	if p.ProofSystem != "groth16" && p.ProofSystem != "plonk" {
		return fmt.Errorf("proof_system must be 'groth16' or 'plonk'")
	}
	if p.MaxMemoSize > 4096 {
		return fmt.Errorf("max_memo_size cannot exceed 4096 bytes")
	}
	if p.NullifierCacheDuration < 0 {
		return fmt.Errorf("nullifier_cache_duration must be non-negative")
	}
	return nil
}