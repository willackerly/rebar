package plugins

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/spec"
)

// JSONSchemaPlugin handles JSON Schema files
type JSONSchemaPlugin struct{}

func (p *JSONSchemaPlugin) Name() string      { return "schema" }
func (p *JSONSchemaPlugin) Extension() string { return ".json" }
func (p *JSONSchemaPlugin) SubDir() string    { return "schemas" }

func (p *JSONSchemaPlugin) Extract(contract *spec.Contract) string {
	// Look for Data, Schema, or Model sections
	for name, content := range contract.Sections {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "data") || strings.Contains(lower, "schema") ||
			strings.Contains(lower, "model") || strings.Contains(lower, "type") {
			extracted := extractCodeFromMarkdown(content, "json", "jsonschema")
			if extracted != "" && strings.Contains(extracted, "$schema") {
				return extracted
			}
		}
	}

	// Check code blocks directly
	for _, block := range contract.CodeBlocks {
		lang := strings.ToLower(block.Language)
		if (lang == "json" || lang == "jsonschema") && strings.Contains(block.Content, "$schema") {
			return block.Content
		}
	}

	return ""
}

func (p *JSONSchemaPlugin) Generate(specContent string, sourceFile string) string {
	basename := filepath.Base(sourceFile)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
	contractID := strings.ToUpper(strings.ReplaceAll(nameWithoutExt, "-", "_"))
	contractID = strings.ReplaceAll(contractID, ".", "_")

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# CONTRACT-%s.1.0\n\n", contractID))
	md.WriteString(fmt.Sprintf("> Auto-generated from %s on %s\n\n", basename, time.Now().Format("2006-01-02")))
	md.WriteString("## Purpose\n\n")
	md.WriteString(fmt.Sprintf("Data schema imported from %s.\n\n", basename))
	md.WriteString("## Data Model\n\n")
	md.WriteString("```json\n")
	md.WriteString(specContent)
	md.WriteString("\n```\n\n")
	md.WriteString("## Implementation\n\n")
	md.WriteString("_TODO: Document implementing files and components._\n\n")
	md.WriteString("## Testing\n\n")
	md.WriteString("_TODO: Document test strategy._\n")

	return md.String()
}

func (p *JSONSchemaPlugin) Detect(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" {
		return false
	}

	text := string(content)
	return strings.Contains(text, "$schema")
}

// extractCodeFromMarkdown is duplicated here to avoid circular import
// TODO: Move to shared utility package
func extractCodeFromMarkdown(markdown string, languages ...string) string {
	lines := strings.Split(markdown, "\n")
	var inBlock bool
	var blockLang string
	var content strings.Builder

	langMap := make(map[string]bool)
	for _, lang := range languages {
		langMap[strings.ToLower(lang)] = true
	}

	codeBlockPattern := "```"

	for _, line := range lines {
		if strings.HasPrefix(line, codeBlockPattern) {
			if !inBlock {
				blockLang = ""
				rest := strings.TrimPrefix(line, codeBlockPattern)
				if rest != "" {
					blockLang = strings.ToLower(strings.TrimSpace(rest))
				}
				if langMap[blockLang] {
					inBlock = true
				}
			} else {
				inBlock = false
				if content.Len() > 0 {
					return content.String()
				}
			}
			continue
		}

		if inBlock {
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	return content.String()
}
