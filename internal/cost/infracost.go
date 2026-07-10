package cost

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// IsInstalled checks whether the infracost CLI binary is available on PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("infracost")
	return err == nil
}

// RunBreakdown runs `infracost breakdown` against a Terraform directory
// and returns parsed per-resource costs.
//
// The directory should contain Terraform files (.tf). Infracost will
// evaluate them and return pricing data from cloud provider APIs.
//
// Requires: infracost CLI installed and configured (INFRACOST_API_KEY set
// or `infracost auth login` completed).
func RunBreakdown(tfDir string) ([]ResourceCost, error) {
	if !IsInstalled() {
		return nil, fmt.Errorf("infracost CLI not found on PATH — install: https://www.infracost.io/docs/")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("infracost", "breakdown",
		"--path", tfDir,
		"--format", "json",
		"--no-color",
		"--log-level", "warn",
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("infracost breakdown failed: %s", errMsg)
	}

	return Parse(stdout.Bytes())
}
