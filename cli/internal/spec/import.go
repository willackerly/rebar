package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Import imports standard specs as REBAR contracts
func Import(opts ImportOptions) error {
	if len(opts.SpecPaths) == 0 {
		return fmt.Errorf("no spec files specified")
	}

	fmt.Printf("Importing %d spec files to %s/\n\n", len(opts.SpecPaths), opts.ContractDir)

	created := 0
	updated := 0
	skipped := 0
	errors := 0

	for _, specPath := range opts.SpecPaths {
		abs, err := filepath.Abs(specPath)
		if err != nil {
			fmt.Printf("  ✗ %s: invalid path\n", specPath)
			errors++
			continue
		}

		rel, _ := filepath.Rel(opts.RepoRoot, abs)
		fmt.Printf("  %s\n", rel)

		// Detect format from extension
		format := detectFormat(abs)
		if format == "" {
			fmt.Printf("    ⚠ skipped: unknown format\n")
			skipped++
			continue
		}

		// Generate contract
		contract, err := generateContract(abs, format, opts)
		if err != nil {
			fmt.Printf("    ✗ %v\n", err)
			errors++
			continue
		}

		// Write contract
		contractPath := filepath.Join(opts.RepoRoot, opts.ContractDir, contract.filename)
		exists := false
		if _, err := os.Stat(contractPath); err == nil {
			exists = true
			if !opts.Force && !opts.DryRun {
				fmt.Printf("    ⚠ skipped: contract exists (use --force to overwrite)\n")
				skipped++
				continue
			}
		}

		if !opts.DryRun {
			if err := os.MkdirAll(filepath.Dir(contractPath), 0755); err != nil {
				fmt.Printf("    ✗ %v\n", err)
				errors++
				continue
			}
			if err := os.WriteFile(contractPath, []byte(contract.content), 0644); err != nil {
				fmt.Printf("    ✗ %v\n", err)
				errors++
				continue
			}
		}

		if exists {
			fmt.Printf("    ✓ updated %s\n", contract.filename)
			updated++
		} else {
			fmt.Printf("    ✓ created %s\n", contract.filename)
			created++
		}
	}

	fmt.Printf("\n✓ Imported %d specs (%d created, %d updated, %d skipped, %d errors)\n", created+updated, created, updated, skipped, errors)
	if opts.DryRun {
		fmt.Println("(dry-run: no files written)")
	}

	return nil
}

type generatedContract struct {
	filename string
	content  string
}

func detectFormat(path string) SpecFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".feature":
		return FormatGherkin
	case ".mmd":
		return FormatMermaid
	case ".yaml", ".yml":
		// Check if it's OpenAPI by reading content
		data, err := os.ReadFile(path)
		if err == nil && (strings.Contains(string(data), "openapi:") || strings.Contains(string(data), "paths:")) {
			return FormatOpenAPI
		}
	case ".json":
		// Check if it's JSON Schema
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), "$schema") {
			return FormatSchema
		}
	}
	return ""
}

func generateContract(specPath string, format SpecFormat, opts ImportOptions) (*generatedContract, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	basename := filepath.Base(specPath)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))

	// Generate contract ID from filename
	contractID := strings.ToUpper(strings.ReplaceAll(nameWithoutExt, "-", "_"))
	contractID = strings.ReplaceAll(contractID, ".", "_")

	var markdown strings.Builder

	// Header
	markdown.WriteString(fmt.Sprintf("# CONTRACT-%s.1.0\n\n", contractID))
	markdown.WriteString(fmt.Sprintf("> Auto-generated from %s on %s\n\n", basename, time.Now().Format("2006-01-02")))

	// Format-specific sections
	switch format {
	case FormatGherkin:
		markdown.WriteString("## Purpose\n\n")
		markdown.WriteString(fmt.Sprintf("Behavior specification imported from %s.\n\n", basename))
		markdown.WriteString("## Scenarios\n\n")
		markdown.WriteString("```gherkin\n")
		markdown.WriteString(content)
		markdown.WriteString("\n```\n\n")

	case FormatMermaid:
		markdown.WriteString("## Purpose\n\n")
		markdown.WriteString(fmt.Sprintf("Architecture diagram imported from %s.\n\n", basename))
		markdown.WriteString("## Architecture\n\n")
		markdown.WriteString("```mermaid\n")
		markdown.WriteString(content)
		markdown.WriteString("\n```\n\n")

	case FormatOpenAPI:
		markdown.WriteString("## Purpose\n\n")
		markdown.WriteString(fmt.Sprintf("API specification imported from %s.\n\n", basename))
		markdown.WriteString("## API\n\n")
		markdown.WriteString("```yaml\n")
		markdown.WriteString(content)
		markdown.WriteString("\n```\n\n")

	case FormatSchema:
		markdown.WriteString("## Purpose\n\n")
		markdown.WriteString(fmt.Sprintf("Data schema imported from %s.\n\n", basename))
		markdown.WriteString("## Data Model\n\n")
		markdown.WriteString("```json\n")
		markdown.WriteString(content)
		markdown.WriteString("\n```\n\n")
	}

	// Footer
	markdown.WriteString("## Implementation\n\n")
	markdown.WriteString("_TODO: Document implementing files and components._\n\n")
	markdown.WriteString("## Testing\n\n")
	markdown.WriteString("_TODO: Document test strategy._\n")

	return &generatedContract{
		filename: fmt.Sprintf("CONTRACT-%s.1.0.md", contractID),
		content:  markdown.String(),
	}, nil
}
