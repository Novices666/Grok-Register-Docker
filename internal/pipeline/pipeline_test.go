package pipeline

import (
	"testing"

	"github.com/grok-free-register/grok-reg/internal/config"
)

func TestShouldRunCPAFlowFollowsCPAOutput(t *testing.T) {
	cfg := config.Defaults()

	if !shouldRunCPAFlow(cfg) {
		t.Fatalf("shouldRunCPAFlow(defaults) = false, want true")
	}

	cfg.OutputCPAEnabled = false
	if shouldRunCPAFlow(cfg) {
		t.Fatalf("shouldRunCPAFlow(output=false) = true, want false")
	}

	cfg.OutputCPAEnabled = true
	if !shouldRunCPAFlow(cfg) {
		t.Fatalf("shouldRunCPAFlow(output=true) = false, want true")
	}
}

func TestUploadRequiresCPAOutputAndUploadFlag(t *testing.T) {
	cfg := config.Defaults()
	cfg.CPAUploadEnabled = true
	cfg.OutputCPAEnabled = true
	if !shouldRunCPAFlow(cfg) || !(shouldRunCPAFlow(cfg) && cfg.CPAUploadEnabled) {
		t.Fatal("upload path should be armed when both CPA output and upload are on")
	}
	cfg.OutputCPAEnabled = false
	if shouldRunCPAFlow(cfg) {
		t.Fatal("CPA flow must stop when OUTPUT_CPA_ENABLED=0")
	}
	// Upload gate mirrors pipeline: shouldRunCPAFlow && CPAUploadEnabled
	if shouldRunCPAFlow(cfg) && cfg.CPAUploadEnabled {
		t.Fatal("upload must not run when CPA output is off")
	}
}

func TestTurnstileMintNeedAllowsTokenForReservedAccount(t *testing.T) {
	if got := turnstileMintNeed(1, 1, 0, 0); got != 1 {
		t.Fatalf("turnstileMintNeed(target=1 reserved=1 done=0 tDepth=0) = %d, want 1", got)
	}

	if got := turnstileMintNeed(1, 1, 0, 1); got != 0 {
		t.Fatalf("turnstileMintNeed(target=1 reserved=1 done=0 tDepth=1) = %d, want 0", got)
	}

	if got := turnstileMintNeed(1, 0, 1, 0); got != 0 {
		t.Fatalf("turnstileMintNeed(target=1 reserved=0 done=1 tDepth=0) = %d, want 0", got)
	}
}
