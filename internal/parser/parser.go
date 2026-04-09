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

	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("unmarshalling plan JSON: %w", err)
	}

	var resources []Resource
	if plan.PlannedValues != nil && plan.PlannedValues.RootModule != nil {
		resources = extractFromModule(plan.PlannedValues.RootModule, "")
	}

	// Attach dependency data from ResourceChanges which has the full dep list.
	depMap := buildDepMap(plan.ResourceChanges)
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
			Attributes: flattenAttributes(r.AttributeValues),
			Module:     modulePrefix,
		}
		res.Tags = extractTags(res.Attributes)
		resources = append(resources, res)
	}

	// Recurse into child modules
	for _, child := range mod.ChildModules {
		prefix := child.Address
		resources = append(resources, extractFromModule(child, prefix)...)
	}

	return resources
}

// buildDepMap builds a map[address][]dependency from ResourceChanges.
func buildDepMap(changes []*tfjson.ResourceChange) map[string][]string {
	m := make(map[string][]string)
	for _, rc := range changes {
		if rc != nil {
			m[rc.Address] = rc.RequiredBy
		}
	}
	return m
}

// flattenAttributes converts tfjson's attribute value map to map[string]any.
func flattenAttributes(vals map[string]json.RawMessage) map[string]any {
	out := make(map[string]any, len(vals))
	for k, v := range vals {
		var parsed any
		if err := json.Unmarshal(v, &parsed); err == nil {
			out[k] = parsed
		}
	}
	return out
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
