package service

import "testing"

func TestTerminalFreezeReconcilesOnlyUnchargedCost(t *testing.T) {
	if got := terminalFreezeChargeRemainder(0.292979, 0.292979); got != 0 {
		t.Fatalf("fully charged remainder = %.6f, want 0", got)
	}
	if got := terminalFreezeChargeRemainder(0.292979, 0.1); got != 0.192979 {
		t.Fatalf("partial charge remainder = %.6f, want 0.192979", got)
	}
	if got := terminalFreezeChargeRemainder(0.292979, 0.5); got != 0 {
		t.Fatalf("overcharged remainder = %.6f, want 0", got)
	}
}
