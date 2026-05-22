package spec

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	contractIDPattern = regexp.MustCompile(`^#\s+CONTRACT[-:]([A-Z0-9-]+\.[0-9.]+)`)
	sectionPattern    = regexp.MustCompile(`^##\s+(.+)`)
	codeBlockPattern  = regexp.MustCompile("^```(\\w+)?")
)

// ParseContract reads and parses a contract markdown file
func ParseContract(path string) (*Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	c := &Contract{
		Path:       path,
		Content:    content,
		Sections:   make(map[string]string),
		CodeBlocks: []CodeBlock{},
	}

	// Extract contract ID and name from first heading
	for _, line := range lines {
		if match := contractIDPattern.FindStringSubmatch(line); match != nil {
			c.ID = match[1]
			parts := strings.SplitN(strings.TrimPrefix(line, "#"), "-", 2)
			if len(parts) > 1 {
				c.Name = strings.TrimSpace(parts[1])
			}
			break
		}
	}

	// Parse sections and code blocks
	var currentSection string
	var sectionContent strings.Builder
	var inCodeBlock bool
	var codeBlockLang string
	var codeBlockContent strings.Builder
	var codeBlockLine int

	for i, line := range lines {
		// Section header
		if match := sectionPattern.FindStringSubmatch(line); match != nil && !inCodeBlock {
			// Save previous section
			if currentSection != "" {
				c.Sections[currentSection] = strings.TrimSpace(sectionContent.String())
				sectionContent.Reset()
			}
			currentSection = strings.TrimSpace(match[1])
			continue
		}

		// Code block start/end
		if match := codeBlockPattern.FindStringSubmatch(line); match != nil {
			if !inCodeBlock {
				// Start code block
				inCodeBlock = true
				codeBlockLang = ""
				if len(match) > 1 {
					codeBlockLang = match[1]
				}
				codeBlockLine = i + 1
				codeBlockContent.Reset()
			} else {
				// End code block
				inCodeBlock = false
				c.CodeBlocks = append(c.CodeBlocks, CodeBlock{
					Language: codeBlockLang,
					Content:  strings.TrimSpace(codeBlockContent.String()),
					Line:     codeBlockLine,
				})
			}
			continue
		}

		// Accumulate content
		if inCodeBlock {
			codeBlockContent.WriteString(line)
			codeBlockContent.WriteString("\n")
		} else if currentSection != "" {
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
		}
	}

	// Save final section
	if currentSection != "" {
		c.Sections[currentSection] = strings.TrimSpace(sectionContent.String())
	}

	return c, nil
}

// FindContracts scans for contract files matching patterns
func FindContracts(contractDir string, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{"CONTRACT-*.md"}
	}

	var contracts []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		// If pattern is absolute or contains path separators, use it directly
		searchPattern := pattern
		if !filepath.IsAbs(pattern) && !strings.Contains(pattern, string(filepath.Separator)) {
			searchPattern = filepath.Join(contractDir, pattern)
		}

		matches, err := filepath.Glob(searchPattern)
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", searchPattern, err)
		}

		for _, match := range matches {
			abs, err := filepath.Abs(match)
			if err != nil {
				continue
			}
			if !seen[abs] {
				contracts = append(contracts, abs)
				seen[abs] = true
			}
		}
	}

	return contracts, nil
}

// ExtractMermaidDiagrams extracts all mermaid code blocks from contract
func (c *Contract) ExtractMermaidDiagrams() []string {
	var diagrams []string
	for _, block := range c.CodeBlocks {
		if block.Language == "mermaid" {
			diagrams = append(diagrams, block.Content)
		}
	}
	return diagrams
}

// ExtractGherkinScenarios extracts Gherkin from "Scenarios" or "Behavior" sections
func (c *Contract) ExtractGherkinScenarios() string {
	// Look for section with scenarios
	for name, content := range c.Sections {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "scenario") || strings.Contains(lower, "behavior") || strings.Contains(lower, "test") {
			// Check if it's already Gherkin-formatted
			if strings.Contains(content, "Feature:") || strings.Contains(content, "Scenario:") {
				return content
			}
			// Convert bullet points to Gherkin
			return convertToGherkin(c.Name, content)
		}
	}

	// Check code blocks for gherkin
	for _, block := range c.CodeBlocks {
		if block.Language == "gherkin" || block.Language == "feature" {
			return block.Content
		}
	}

	return ""
}

// ExtractOpenAPISpec extracts OpenAPI from "API" section or openapi/yaml code blocks
func (c *Contract) ExtractOpenAPISpec() string {
	// Check for API section
	for name, content := range c.Sections {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "api") || strings.Contains(lower, "endpoint") {
			// Look for code block within section content
			if strings.Contains(content, "```") {
				return extractCodeFromMarkdown(content, "yaml", "yml", "openapi")
			}
		}
	}

	// Check code blocks
	for _, block := range c.CodeBlocks {
		lang := strings.ToLower(block.Language)
		if lang == "yaml" || lang == "openapi" || lang == "yml" {
			if strings.Contains(block.Content, "paths:") || strings.Contains(block.Content, "openapi:") {
				return block.Content
			}
		}
	}

	return ""
}

// ExtractJSONSchema extracts JSON schemas from "Data" or "Schema" sections
func (c *Contract) ExtractJSONSchema() string {
	for name, content := range c.Sections {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "data") || strings.Contains(lower, "schema") || strings.Contains(lower, "model") {
			return extractCodeFromMarkdown(content, "json", "jsonschema")
		}
	}

	for _, block := range c.CodeBlocks {
		lang := strings.ToLower(block.Language)
		if (lang == "json" || lang == "jsonschema") && strings.Contains(block.Content, "$schema") {
			return block.Content
		}
	}

	return ""
}

// convertToGherkin converts bullet-point scenarios to Gherkin format
func convertToGherkin(featureName, content string) string {
	var gherkin strings.Builder
	gherkin.WriteString(fmt.Sprintf("Feature: %s\n\n", featureName))

	scanner := bufio.NewScanner(strings.NewReader(content))
	scenarioNum := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Bullet points become Given/When/Then
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			step := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")

			// Heuristic mapping
			lower := strings.ToLower(step)
			if strings.HasPrefix(lower, "given ") || strings.HasPrefix(lower, "when ") || strings.HasPrefix(lower, "then ") {
				gherkin.WriteString(fmt.Sprintf("    %s\n", step))
			} else if strings.Contains(lower, "should") || strings.Contains(lower, "must") {
				gherkin.WriteString(fmt.Sprintf("    Then %s\n", step))
			} else {
				gherkin.WriteString(fmt.Sprintf("    Given %s\n", step))
			}
		} else if !strings.HasPrefix(line, "#") {
			// New scenario
			gherkin.WriteString(fmt.Sprintf("  Scenario: %s\n", line))
			scenarioNum++
		}
	}

	return gherkin.String()
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

	for _, line := range lines {
		if match := codeBlockPattern.FindStringSubmatch(line); match != nil {
			if !inBlock {
				blockLang = ""
				if len(match) > 1 {
					blockLang = strings.ToLower(match[1])
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
