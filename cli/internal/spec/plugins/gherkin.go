package plugins

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/spec"
)

// GherkinPlugin handles Gherkin/Cucumber feature files
type GherkinPlugin struct{}

func (p *GherkinPlugin) Name() string      { return "gherkin" }
func (p *GherkinPlugin) Extension() string { return ".feature" }
func (p *GherkinPlugin) SubDir() string    { return "gherkin" }

func (p *GherkinPlugin) Extract(contract *spec.Contract) string {
	// Look for Scenarios/Behavior section
	for name, content := range contract.Sections {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "scenario") || strings.Contains(lower, "behavior") || strings.Contains(lower, "test") {
			// Check if already Gherkin-formatted
			if strings.Contains(content, "Feature:") || strings.Contains(content, "Scenario:") {
				return content
			}
			// Convert bullet points to Gherkin
			return convertToGherkin(contract.Name, content)
		}
	}

	// Check code blocks for gherkin
	for _, block := range contract.CodeBlocks {
		if block.Language == "gherkin" || block.Language == "feature" {
			return block.Content
		}
	}

	return ""
}

func (p *GherkinPlugin) Generate(specContent string, sourceFile string) string {
	basename := filepath.Base(sourceFile)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
	contractID := strings.ToUpper(strings.ReplaceAll(nameWithoutExt, "-", "_"))
	contractID = strings.ReplaceAll(contractID, ".", "_")

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# CONTRACT-%s.1.0\n\n", contractID))
	md.WriteString(fmt.Sprintf("> Auto-generated from %s on %s\n\n", basename, time.Now().Format("2006-01-02")))
	md.WriteString("## Purpose\n\n")
	md.WriteString(fmt.Sprintf("Behavior specification imported from %s.\n\n", basename))
	md.WriteString("## Scenarios\n\n")
	md.WriteString("```gherkin\n")
	md.WriteString(specContent)
	md.WriteString("\n```\n\n")
	md.WriteString("## Implementation\n\n")
	md.WriteString("_TODO: Document implementing files and components._\n\n")
	md.WriteString("## Testing\n\n")
	md.WriteString("_TODO: Document test strategy._\n")

	return md.String()
}

func (p *GherkinPlugin) Detect(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".feature" {
		return true
	}

	// Check content for Gherkin keywords
	text := string(content)
	return strings.Contains(text, "Feature:") && (strings.Contains(text, "Scenario:") || strings.Contains(text, "Given "))
}

// convertToGherkin converts bullet-point scenarios to proper Gherkin format
func convertToGherkin(featureName, content string) string {
	var gherkin strings.Builder
	gherkin.WriteString(fmt.Sprintf("Feature: %s\n\n", featureName))

	scanner := bufio.NewScanner(strings.NewReader(content))
	inScenario := false
	scenarioNum := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if inScenario {
				gherkin.WriteString("\n")
			}
			continue
		}

		// Check if line is a Given/When/Then step
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "given ") || strings.HasPrefix(lower, "when ") ||
			strings.HasPrefix(lower, "then ") || strings.HasPrefix(lower, "and ") ||
			strings.HasPrefix(lower, "but ") {
			// Capitalize first letter
			line = strings.ToUpper(string(line[0])) + line[1:]
			gherkin.WriteString(fmt.Sprintf("    %s\n", line))
			inScenario = true
			continue
		}

		// Bullet points become steps
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			step := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			step = strings.TrimSpace(step)

			// Heuristic mapping based on keywords
			lower := strings.ToLower(step)
			if strings.Contains(lower, "should") || strings.Contains(lower, "must") ||
				strings.Contains(lower, "expect") || strings.Contains(lower, "verify") {
				gherkin.WriteString(fmt.Sprintf("    Then %s\n", step))
			} else if strings.Contains(lower, "when") || strings.Contains(lower, "trigger") ||
				strings.Contains(lower, "click") || strings.Contains(lower, "submit") {
				gherkin.WriteString(fmt.Sprintf("    When %s\n", step))
			} else {
				gherkin.WriteString(fmt.Sprintf("    Given %s\n", step))
			}
			inScenario = true
		} else if !strings.HasPrefix(line, "#") {
			// Non-bullet, non-comment line = scenario title
			if inScenario {
				gherkin.WriteString("\n")
			}
			gherkin.WriteString(fmt.Sprintf("  Scenario: %s\n", line))
			inScenario = true
			scenarioNum++
		}
	}

	return gherkin.String()
}
