// Package threat detects security misconfigurations in Terraform resources.
// It analyses resource attributes from parsed plan/state files and produces
// a list of Findings — each tied to a specific resource address.
//
// Severity levels:
//
//	Critical  — direct exposure to internet or data loss risk
//	High      — significant security weakness, should be fixed before deploy
//	Medium    — best practice violation, fix soon
//	Info      — informational, may be intentional
package threat

// Severity indicates how serious a finding is.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityInfo     Severity = "info"
)

// Finding is a single detected security issue on a resource.
type Finding struct {
	// ResourceAddress is the Terraform address, e.g. "aws_instance.web"
	ResourceAddress string
	// ResourceType is the Terraform type, e.g. "aws_instance"
	ResourceType string
	// Severity of the finding
	Severity Severity
	// Code is a short machine-readable identifier, e.g. "SG001"
	Code string
	// Title is a short human-readable description
	Title string
	// Detail explains exactly what was found and why it matters
	Detail string
	// Remediation is a concrete fix suggestion
	Remediation string
}

// SeverityWeight returns a numeric weight for sorting (higher = more severe).
func (s Severity) Weight() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityInfo:
		return 1
	}
	return 0
}

// SeverityColor returns the hex color for UI display.
func (s Severity) Color() string {
	switch s {
	case SeverityCritical:
		return "#C53030" // red
	case SeverityHigh:
		return "#C05621" // orange
	case SeverityMedium:
		return "#975A16" // amber
	case SeverityInfo:
		return "#2B6CB0" // blue
	}
	return "#718096"
}

// SeverityIcon returns an emoji icon for terminal output.
func (s Severity) Icon() string {
	switch s {
	case SeverityCritical:
		return "🔴"
	case SeverityHigh:
		return "🟠"
	case SeverityMedium:
		return "🟡"
	case SeverityInfo:
		return "🔵"
	}
	return "⚪"
}