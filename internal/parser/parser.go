// Package parser reads Terraform plan and state JSON files and returns
// a normalised []Resource slice for the graph engine.
package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// rawPlanDeps is used to extract the `required_by` field from resource_changes,
// which the terraform-json library does not expose as a typed field.
type rawPlanDeps struct {
	ResourceChanges []struct {
		Address    string   `json:"address"`
		RequiredBy []string `json:"required_by"`
		DependsOn  []string `json:"depends_on"`
	} `json:"resource_changes"`
}

// rawConfigDeps extracts depends_on from the plan configuration block.
type rawConfigDeps struct {
	Configuration struct {
		RootModule struct {
			Resources []struct {
				Address   string   `json:"address"`
				DependsOn []string `json:"depends_on"`
			} `json:"resources"`
		} `json:"root_module"`
	} `json:"configuration"`
}

// Resource is the normalised representation of a single Terraform resource.
// Both plan and state files are reduced to this common model.
type Resource struct {
	// Address is the fully qualified resource address, e.g. "aws_instance.web"
	Address string
	// Type is the Terraform resource type, e.g. "aws_instance"
	Type string
	// Name is the user-defined resource name, e.g. "web"
	Name string
	// Provider is the short provider name, e.g. "aws"
	Provider string
	// Attributes holds the resource's configuration attributes.
	Attributes map[string]any
	// Dependencies lists addresses of resources this resource depends on.
	Dependencies []string
	// Tags are extracted from the "tags" attribute for display.
	Tags map[string]string
	// Module is non-empty when the resource belongs to a Terraform module.
	Module string
}

// ParsePlanFile parses a JSON file produced by `terraform show -json <planfile>`
// or `terraform plan -out=plan.bin && terraform show -json plan.bin`.
func ParsePlanFile(path string) ([]Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file: %w", err)
	}

	// Use the typed library for planned_values (resource attributes).
	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("unmarshalling plan JSON: %w", err)
	}

	var resources []Resource
	if plan.PlannedValues != nil && plan.PlannedValues.RootModule != nil {
		resources = extractFromModule(plan.PlannedValues.RootModule, "")
	}

	// The terraform-json library does not expose required_by / depends_on on
	// ResourceChange. Parse those from the raw JSON directly.
	depMap := buildDepMapFromRaw(data)
	for i := range resources {
		if deps, ok := depMap[resources[i].Address]; ok {
			resources[i].Dependencies = deps
		}
	}

	return resources, nil
}

// ParseStateFile parses a terraform.tfstate JSON file.
func ParseStateFile(path string) ([]Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state tfjson.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshalling state JSON: %w", err)
	}

	var resources []Resource
	if state.Values != nil && state.Values.RootModule != nil {
		resources = extractFromModule(state.Values.RootModule, "")
	}

	return resources, nil
}

// ---- internal helpers -------------------------------------------------------

// extractFromModule recursively walks the terraform module tree.
func extractFromModule(mod *tfjson.StateModule, modulePrefix string) []Resource {
	var resources []Resource

	for _, r := range mod.Resources {
		res := Resource{
			Address:    r.Address,
			Type:       r.Type,
			Name:       r.Name,
			Provider:   providerShortName(r.ProviderName),
			Attributes: r.AttributeValues, // already map[string]interface{}
			Module:     modulePrefix,
		}
		res.Tags = extractTags(res.Attributes)
		resources = append(resources, res)
	}

	// Recurse into child modules
	for _, child := range mod.ChildModules {
		resources = append(resources, extractFromModule(child, child.Address)...)
	}

	return resources
}

// buildDepMapFromRaw extracts dependency data from the raw plan JSON.
// terraform-json does not expose required_by or depends_on as typed fields,
// so we unmarshal only the parts we need from the raw bytes.
func buildDepMapFromRaw(data []byte) map[string][]string {
	m := make(map[string][]string)

	// Try resource_changes[].required_by first (present in some plan formats).
	var rcd rawPlanDeps
	if err := json.Unmarshal(data, &rcd); err == nil {
		for _, rc := range rcd.ResourceChanges {
			if len(rc.RequiredBy) > 0 {
				m[rc.Address] = rc.RequiredBy
			} else if len(rc.DependsOn) > 0 {
				m[rc.Address] = rc.DependsOn
			}
		}
	}

	// Also try configuration.root_module.resources[].depends_on as a fallback.
	var cfg rawConfigDeps
	if err := json.Unmarshal(data, &cfg); err == nil {
		for _, r := range cfg.Configuration.RootModule.Resources {
			if _, already := m[r.Address]; !already && len(r.DependsOn) > 0 {
				m[r.Address] = r.DependsOn
			}
		}
	}

	return m
}

// flattenAttributes is kept for compatibility but is no longer used for plan files.
// StateResource.AttributeValues is already map[string]interface{} in terraform-json.
func flattenAttributes(vals map[string]interface{}) map[string]interface{} {
	return vals
}

// extractTags pulls the "tags" attribute into a clean map[string]string.
func extractTags(attrs map[string]any) map[string]string {
	tags := make(map[string]string)
	raw, ok := attrs["tags"]
	if !ok {
		return tags
	}
	switch t := raw.(type) {
	case map[string]any:
		for k, v := range t {
			if s, ok := v.(string); ok {
				tags[k] = s
			}
		}
	}
	return tags
}

// providerShortName strips the registry prefix from a provider name.
// "registry.terraform.io/hashicorp/aws" → "aws"
func providerShortName(full string) string {
	parts := strings.Split(full, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return full
}