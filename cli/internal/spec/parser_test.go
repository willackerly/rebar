package spec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseContract(t *testing.T) {
	contractPath := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")

	contract, err := ParseContract(contractPath)
	if err != nil {
		t.Fatalf("ParseContract() error = %v", err)
	}

	if contract.ID != "AUTH.1.0" {
		t.Errorf("ID = %q, want %q", contract.ID, "AUTH.1.0")
	}

	if contract.Path != contractPath {
		t.Errorf("Path = %q, want %q", contract.Path, contractPath)
	}

	// Check sections
	expectedSections := []string{"Purpose", "Scenarios", "Architecture", "API", "Data Model", "Implementation", "Testing"}
	for _, section := range expectedSections {
		if _, ok := contract.Sections[section]; !ok {
			t.Errorf("Missing section: %s", section)
		}
	}

	// Check code blocks
	if len(contract.CodeBlocks) < 3 {
		t.Errorf("CodeBlocks count = %d, want at least 3 (mermaid, yaml, json)", len(contract.CodeBlocks))
	}

	foundMermaid := false
	foundYAML := false
	foundJSON := false
	for _, block := range contract.CodeBlocks {
		switch block.Language {
		case "mermaid":
			foundMermaid = true
		case "yaml":
			foundYAML = true
		case "json":
			foundJSON = true
		}
	}

	if !foundMermaid {
		t.Error("No mermaid code block found")
	}
	if !foundYAML {
		t.Error("No yaml code block found")
	}
	if !foundJSON {
		t.Error("No json code block found")
	}
}

func TestExtractMermaidDiagrams(t *testing.T) {
	contractPath := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")
	contract, err := ParseContract(contractPath)
	if err != nil {
		t.Fatalf("ParseContract() error = %v", err)
	}

	diagrams := contract.ExtractMermaidDiagrams()
	if len(diagrams) == 0 {
		t.Error("ExtractMermaidDiagrams() returned no diagrams")
	}

	if len(diagrams) > 0 && !contains(diagrams[0], "graph TD") {
		t.Error("Mermaid diagram doesn't contain expected content")
	}
}

func TestExtractGherkinScenarios(t *testing.T) {
	contractPath := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")
	contract, err := ParseContract(contractPath)
	if err != nil {
		t.Fatalf("ParseContract() error = %v", err)
	}

	gherkin := contract.ExtractGherkinScenarios()
	if gherkin == "" {
		t.Error("ExtractGherkinScenarios() returned empty string")
		return
	}

	if !contains(gherkin, "Feature:") {
		t.Errorf("Gherkin doesn't contain Feature keyword. Got:\n%s", gherkin)
		return
	}

	if !contains(gherkin, "Given") {
		t.Errorf("Gherkin doesn't contain Given step. Got:\n%s", gherkin)
	}
}

func TestExtractOpenAPISpec(t *testing.T) {
	contractPath := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")
	contract, err := ParseContract(contractPath)
	if err != nil {
		t.Fatalf("ParseContract() error = %v", err)
	}

	openapi := contract.ExtractOpenAPISpec()
	if openapi == "" {
		t.Error("ExtractOpenAPISpec() returned empty string")
	}

	if !contains(openapi, "openapi:") && !contains(openapi, "paths:") {
		t.Error("OpenAPI spec doesn't contain expected keywords")
	}
}

func TestExtractJSONSchema(t *testing.T) {
	contractPath := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")
	contract, err := ParseContract(contractPath)
	if err != nil {
		t.Fatalf("ParseContract() error = %v", err)
	}

	schema := contract.ExtractJSONSchema()
	if schema == "" {
		t.Error("ExtractJSONSchema() returned empty string")
	}

	if !contains(schema, "$schema") {
		t.Error("JSON schema doesn't contain $schema keyword")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
