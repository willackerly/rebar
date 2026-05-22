package plugins

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/spec"
)

// OpenAPIPlugin handles OpenAPI specification files
type OpenAPIPlugin struct{}

func (p *OpenAPIPlugin) Name() string      { return "openapi" }
func (p *OpenAPIPlugin) Extension() string { return ".yaml" }
func (p *OpenAPIPlugin) SubDir() string    { return "openapi" }

func (p *OpenAPIPlugin) Extract(contract *spec.Contract) string {
	// Check for API section
	for name, content := range contract.Sections {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "api") || strings.Contains(lower, "endpoint") {
			// Extract code block from section
			extracted := extractCodeFromMarkdown(content, "yaml", "yml", "openapi")
			if extracted != "" {
				return extracted
			}
		}
	}

	// Check code blocks directly
	for _, block := range contract.CodeBlocks {
		lang := strings.ToLower(block.Language)
		if lang == "yaml" || lang == "openapi" || lang == "yml" {
			if strings.Contains(block.Content, "paths:") || strings.Contains(block.Content, "openapi:") {
				return block.Content
			}
		}
	}

	return ""
}

func (p *OpenAPIPlugin) Generate(specContent string, sourceFile string) string {
	basename := filepath.Base(sourceFile)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
	contractID := strings.ToUpper(strings.ReplaceAll(nameWithoutExt, "-", "_"))
	contractID = strings.ReplaceAll(contractID, ".", "_")

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# CONTRACT-%s.1.0\n\n", contractID))
	md.WriteString(fmt.Sprintf("> Auto-generated from %s on %s\n\n", basename, time.Now().Format("2006-01-02")))
	md.WriteString("## Purpose\n\n")
	md.WriteString(fmt.Sprintf("API specification imported from %s.\n\n", basename))
	md.WriteString("## API\n\n")
	md.WriteString("```yaml\n")
	md.WriteString(specContent)
	md.WriteString("\n```\n\n")
	md.WriteString("## Implementation\n\n")
	md.WriteString("_TODO: Document implementing files and components._\n\n")
	md.WriteString("## Testing\n\n")
	md.WriteString("_TODO: Document test strategy._\n")

	return md.String()
}

func (p *OpenAPIPlugin) Detect(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return false
	}

	text := string(content)
	return strings.Contains(text, "openapi:") || strings.Contains(text, "paths:")
}

// extractCodeFromMarkdown extracts code blocks of specific languages from markdown
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
