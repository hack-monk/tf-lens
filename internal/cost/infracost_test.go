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

func TestRunBreakdownForPlanMissingBinary(t *testing.T) {
	if IsInstalled() {
		t.Skip("infracost is installed — cannot test missing binary path")
	}
	_, err := RunBreakdownForPlan("/tmp/fake.json")
	if err == nil {
		t.Fatal("expected error when infracost not installed")
	}
}

func TestVersionMissingBinary(t *testing.T) {
	if IsInstalled() {
		t.Skip("infracost is installed — cannot test missing binary path")
	}
	_, err := Version()
	if err == nil {
		t.Fatal("expected error when infracost not installed")
	}
}
