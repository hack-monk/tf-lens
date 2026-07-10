package cost

import (
	"testing"
)

func TestIsInstalled(t *testing.T) {
	// Just verify it doesn't panic — result depends on environment
	_ = IsInstalled()
}

func TestRunBreakdownMissingBinary(t *testing.T) {
	if IsInstalled() {
		t.Skip("infracost is installed — cannot test missing binary path")
	}
	_, err := RunBreakdown("/tmp")
	if err == nil {
		t.Fatal("expected error when infracost not installed")
	}
}
