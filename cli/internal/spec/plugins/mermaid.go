package plugins

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/spec"
)

// MermaidPlugin handles Mermaid diagram files
type MermaidPlugin struct{}

func (p *MermaidPlugin) Name() string      { return "mermaid" }
func (p *MermaidPlugin) Extension() string { return ".mmd" }
func (p *MermaidPlugin) SubDir() string    { return "mermaid" }

func (p *MermaidPlugin) Extract(contract *spec.Contract) string {
	diagrams := []string{}
	for _, block := range contract.CodeBlocks {
		if block.Language == "mermaid" {
			diagrams = append(diagrams, block.Content)
		}
	}

	if len(diagrams) == 0 {
		return ""
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

	return combined.String()
}

func (p *MermaidPlugin) Generate(specContent string, sourceFile string) string {
	basename := filepath.Base(sourceFile)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
	contractID := strings.ToUpper(strings.ReplaceAll(nameWithoutExt, "-", "_"))
	contractID = strings.ReplaceAll(contractID, ".", "_")

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# CONTRACT-%s.1.0\n\n", contractID))
	md.WriteString(fmt.Sprintf("> Auto-generated from %s on %s\n\n", basename, time.Now().Format("2006-01-02")))
	md.WriteString("## Purpose\n\n")
	md.WriteString(fmt.Sprintf("Architecture diagram imported from %s.\n\n", basename))
	md.WriteString("## Architecture\n\n")
	md.WriteString("```mermaid\n")
	md.WriteString(specContent)
	md.WriteString("\n```\n\n")
	md.WriteString("## Implementation\n\n")
	md.WriteString("_TODO: Document implementing files and components._\n\n")
	md.WriteString("## Testing\n\n")
	md.WriteString("_TODO: Document test strategy._\n")

	return md.String()
}

func (p *MermaidPlugin) Detect(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mmd" || ext == ".mermaid" {
		return true
	}

	// Check content for Mermaid diagram types
	text := strings.ToLower(string(content))
	return strings.Contains(text, "graph ") || strings.Contains(text, "sequenceDiagram") ||
		strings.Contains(text, "classDiagram") || strings.Contains(text, "erDiagram") ||
		strings.Contains(text, "flowchart ")
}
