package parser_test

import (
	"os"
	"testing"

	"github.com/hack-monk/tf-lens/internal/parser"
)

func TestParsePlanFile_BasicFixture(t *testing.T) {
	resources, err := parser.ParsePlanFile("../../testdata/plan_basic.json")
	if err != nil {
		t.Fatalf("ParsePlanFile: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("expected resources, got none")
	}
	byAddr := make(map[string]parser.Resource, len(resources))
	for _, r := range resources {
		byAddr[r.Address] = r
	}
	vpc, ok := byAddr["aws_vpc.main"]
	if !ok {
		t.Fatal("aws_vpc.main not found")
	}
	if vpc.Type != "aws_vpc" {
		t.Errorf("vpc.Type = %q, want aws_vpc", vpc.Type)
	}
	if vpc.Provider != "aws" {
		t.Errorf("vpc.Provider = %q, want aws", vpc.Provider)
	}
	if vpc.Tags["Name"] != "main-vpc" {
		t.Errorf("vpc tag Name = %q, want main-vpc", vpc.Tags["Name"])
	}
	web, ok := byAddr["aws_instance.web"]
	if !ok {
		t.Fatal("aws_instance.web not found")
	}
	if len(web.Dependencies) == 0 {
		t.Error("expected dependencies on aws_instance.web")
	}
	t.Logf("Parsed %d resources", len(resources))
}

func TestParsePlanFile_ResourceCount(t *testing.T) {
	resources, err := parser.ParsePlanFile("../../testdata/plan_basic.json")
	if err != nil {
		t.Fatalf("ParsePlanFile: %v", err)
	}
	if len(resources) != 12 {
		t.Errorf("resource count = %d, want 12", len(resources))
	}
}

func TestParsePlanFile_TagExtraction(t *testing.T) {
	resources, err := parser.ParsePlanFile("../../testdata/plan_basic.json")
	if err != nil {
		t.Fatalf("ParsePlanFile: %v", err)
	}
	for _, r := range resources {
		if r.Address == "aws_instance.web" {
			if r.Tags["Role"] != "frontend" {
				t.Errorf("expected tag Role=frontend, got %q", r.Tags["Role"])
			}
			return
		}
	}
	t.Error("aws_instance.web not found")
}

func TestParsePlanFile_MissingFile(t *testing.T) {
	_, err := parser.ParsePlanFile("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParsePlanFile_InvalidJSON(t *testing.T) {
	f, err := os.CreateTemp("", "tf-lens-test-*.json")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Write([]byte("not-valid-json"))
	f.Close()
	_, err = parser.ParsePlanFile(f.Name())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
