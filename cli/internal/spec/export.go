package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Export exports contracts to standard specification formats
func Export(opts ExportOptions) error {
	// Find contracts to export
	contractPaths, err := FindContracts(filepath.Join(opts.RepoRoot, opts.ContractDir), opts.Patterns)
	if err != nil {
		return fmt.Errorf("finding contracts: %w", err)
	}

	if len(contractPaths) == 0 {
		fmt.Println("No contracts found to export")
		return nil
	}

	// Load manifest
	manifest, err := LoadManifest(filepath.Join(opts.RepoRoot, opts.OutDir))
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	// Track results
	exported := 0
	skipped := 0
	errors := 0

	fmt.Printf("Exporting %d contracts to %s/\n\n", len(contractPaths), opts.OutDir)

	for _, contractPath := range contractPaths {
		rel, _ := filepath.Rel(opts.RepoRoot, contractPath)
		fmt.Printf("  %s\n", rel)

		// Parse contract
		contract, err := ParseContract(contractPath)
		if err != nil {
			fmt.Printf("    ✗ parse error: %v\n", err)
			errors++
			continue
		}

		if contract.ID == "" {
			fmt.Printf("    ⚠ skipped: no contract ID found\n")
			skipped++
			continue
		}

		// Export to each format
		mapping := SpecMapping{
			Contract: rel,
			Exports:  []ExportedSpec{},
		}

		// Gherkin
		if opts.Format == "" || opts.Format == string(FormatGherkin) {
			if exported, spec := exportGherkin(contract, opts); exported != nil {
				mapping.Exports = append(mapping.Exports, *exported)
				fmt.Printf("    ✓ %s\n", spec)
			}
		}

		// Mermaid
		if opts.Format == "" || opts.Format == string(FormatMermaid) {
			if exported, spec := exportMermaid(contract, opts); exported != nil {
				mapping.Exports = append(mapping.Exports, *exported)
				fmt.Printf("    ✓ %s\n", spec)
			}
		}

		// OpenAPI
		if opts.Format == "" || opts.Format == string(FormatOpenAPI) {
			if exported, spec := exportOpenAPI(contract, opts); exported != nil {
				mapping.Exports = append(mapping.Exports, *exported)
				fmt.Printf("    ✓ %s\n", spec)
			}
		}

		// JSON Schema
		if opts.Format == "" || opts.Format == string(FormatSchema) {
			if exported, spec := exportJSONSchema(contract, opts); exported != nil {
				mapping.Exports = append(mapping.Exports, *exported)
				fmt.Printf("    ✓ %s\n", spec)
			}
		}

		// Update checksums
		if len(mapping.Exports) > 0 {
			mapping.ContractChecksum, _ = ComputeChecksum(contractPath)
			for i := range mapping.Exports {
				absPath := filepath.Join(opts.RepoRoot, mapping.Exports[i].Path)
				mapping.Exports[i].Checksum, _ = ComputeChecksum(absPath)
			}
			manifest.UpdateMapping(mapping)
			exported++
		} else {
			fmt.Printf("    ⚠ no exportable content found\n")
			skipped++
		}
	}

	// Save manifest
	if !opts.DryRun {
		if err := SaveManifest(filepath.Join(opts.RepoRoot, opts.OutDir), manifest); err != nil {
			return fmt.Errorf("saving manifest: %w", err)
		}
	}

	fmt.Printf("\n✓ Exported %d contracts (%d skipped, %d errors)\n", exported, skipped, errors)
	if opts.DryRun {
		fmt.Println("(dry-run: no files written)")
	}

	return nil
}

func exportGherkin(contract *Contract, opts ExportOptions) (*ExportedSpec, string) {
	content := contract.ExtractGherkinScenarios()
	if content == "" {
		return nil, ""
	}

	filename := fmt.Sprintf("%s.feature", sanitizeFilename(contract.ID))
	relPath := filepath.Join(opts.OutDir, "gherkin", filename)
	absPath := filepath.Join(opts.RepoRoot, relPath)

	if !opts.DryRun {
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return nil, ""
		}
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return nil, ""
		}
	}

	return &ExportedSpec{Type: string(FormatGherkin), Path: relPath}, relPath
}

func exportMermaid(contract *Contract, opts ExportOptions) (*ExportedSpec, string) {
	diagrams := contract.ExtractMermaidDiagrams()
	if len(diagrams) == 0 {
		return nil, ""
	}

	// Combine multiple diagrams with comments
	var combined strings.Builder
	for i, diagram := range diagrams {
		if i > 0 {
			combined.WriteString("\n\n%% Diagram ")
			combined.WriteString(fmt.Sprintf("%d", i+1))
			combined.WriteString("\n\n")
		}
		combined.WriteString(diagram)
	}

	filename := fmt.Sprintf("%s.mmd", sanitizeFilename(contract.ID))
	relPath := filepath.Join(opts.OutDir, "mermaid", filename)
	absPath := filepath.Join(opts.RepoRoot, relPath)

	if !opts.DryRun {
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return nil, ""
		}
		if err := os.WriteFile(absPath, []byte(combined.String()), 0644); err != nil {
			return nil, ""
		}
	}

	return &ExportedSpec{Type: string(FormatMermaid), Path: relPath}, relPath
}

func exportOpenAPI(contract *Contract, opts ExportOptions) (*ExportedSpec, string) {
	content := contract.ExtractOpenAPISpec()
	if content == "" {
		return nil, ""
	}

	filename := fmt.Sprintf("%s.yaml", sanitizeFilename(contract.ID))
	relPath := filepath.Join(opts.OutDir, "openapi", filename)
	absPath := filepath.Join(opts.RepoRoot, relPath)

	if !opts.DryRun {
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return nil, ""
		}
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return nil, ""
		}
	}

	return &ExportedSpec{Type: string(FormatOpenAPI), Path: relPath}, relPath
}

func exportJSONSchema(contract *Contract, opts ExportOptions) (*ExportedSpec, string) {
	content := contract.ExtractJSONSchema()
	if content == "" {
		return nil, ""
	}

	filename := fmt.Sprintf("%s.json", sanitizeFilename(contract.ID))
	relPath := filepath.Join(opts.OutDir, "schemas", filename)
	absPath := filepath.Join(opts.RepoRoot, relPath)

	if !opts.DryRun {
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return nil, ""
		}
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return nil, ""
		}
	}

	return &ExportedSpec{Type: string(FormatSchema), Path: relPath}, relPath
}

func sanitizeFilename(s string) string {
	// Replace path separators and special chars
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}
