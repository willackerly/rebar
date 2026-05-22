package spec

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// FormatPlugin defines the interface for spec format handlers
type FormatPlugin interface {
	// Name returns the format identifier (e.g., "gherkin", "mermaid")
	Name() string

	// Extension returns the file extension (e.g., ".feature", ".mmd")
	Extension() string

	// SubDir returns the subdirectory name (e.g., "gherkin", "mermaid")
	SubDir() string

	// Extract extracts spec content from a contract
	Extract(contract *Contract) string

	// Generate generates contract markdown from spec content
	Generate(spec string, sourceFile string) string

	// Detect determines if a file is this format
	Detect(path string, content []byte) bool
}

// PluginRegistry manages available format plugins
type PluginRegistry struct {
	plugins map[string]FormatPlugin
}

// NewPluginRegistry creates a registry with default plugins
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]FormatPlugin),
	}
}

// Register adds a plugin to the registry
func (r *PluginRegistry) Register(plugin FormatPlugin) {
	r.plugins[plugin.Name()] = plugin
}

// Get retrieves a plugin by name
func (r *PluginRegistry) Get(name string) FormatPlugin {
	return r.plugins[name]
}

// All returns all registered plugins
func (r *PluginRegistry) All() []FormatPlugin {
	plugins := make([]FormatPlugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// DetectFormat auto-detects format from file path and content
func (r *PluginRegistry) DetectFormat(path string, content []byte) FormatPlugin {
	for _, plugin := range r.plugins {
		if plugin.Detect(path, content) {
			return plugin
		}
	}
	return nil
}

// DefaultRegistry returns a registry with all standard plugins
func DefaultRegistry() *PluginRegistry {
	r := NewPluginRegistry()
	r.Register(&gherkinPluginImpl{})
	r.Register(&mermaidPluginImpl{})
	r.Register(&openapiPluginImpl{})
	r.Register(&jsonschemaPluginImpl{})
	return r
}

// Built-in plugin implementations
// (Defined here to avoid circular imports with plugins/ package)

type gherkinPluginImpl struct{}

func (p *gherkinPluginImpl) Name() string      { return "gherkin" }
func (p *gherkinPluginImpl) Extension() string { return ".feature" }
func (p *gherkinPluginImpl) SubDir() string    { return "gherkin" }

func (p *gherkinPluginImpl) Extract(contract *Contract) string {
	return contract.ExtractGherkinScenarios()
}

func (p *gherkinPluginImpl) Generate(spec string, sourceFile string) string {
	return generateContractFromSpec("gherkin", spec, sourceFile)
}

func (p *gherkinPluginImpl) Detect(path string, content []byte) bool {
	return detectGherkin(path, content)
}

type mermaidPluginImpl struct{}

func (p *mermaidPluginImpl) Name() string      { return "mermaid" }
func (p *mermaidPluginImpl) Extension() string { return ".mmd" }
func (p *mermaidPluginImpl) SubDir() string    { return "mermaid" }

func (p *mermaidPluginImpl) Extract(contract *Contract) string {
	diagrams := contract.ExtractMermaidDiagrams()
	if len(diagrams) == 0 {
		return ""
	}
	// Combine multiple diagrams
	var combined string
	for i, d := range diagrams {
		if i > 0 {
			combined += "\n\n%% Diagram " + string(rune(i+1)) + "\n\n"
		}
		combined += d
	}
	return combined
}

func (p *mermaidPluginImpl) Generate(spec string, sourceFile string) string {
	return generateContractFromSpec("mermaid", spec, sourceFile)
}

func (p *mermaidPluginImpl) Detect(path string, content []byte) bool {
	return detectMermaid(path, content)
}

type openapiPluginImpl struct{}

func (p *openapiPluginImpl) Name() string      { return "openapi" }
func (p *openapiPluginImpl) Extension() string { return ".yaml" }
func (p *openapiPluginImpl) SubDir() string    { return "openapi" }

func (p *openapiPluginImpl) Extract(contract *Contract) string {
	return contract.ExtractOpenAPISpec()
}

func (p *openapiPluginImpl) Generate(spec string, sourceFile string) string {
	return generateContractFromSpec("openapi", spec, sourceFile)
}

func (p *openapiPluginImpl) Detect(path string, content []byte) bool {
	return detectOpenAPI(path, content)
}

type jsonschemaPluginImpl struct{}

func (p *jsonschemaPluginImpl) Name() string      { return "schema" }
func (p *jsonschemaPluginImpl) Extension() string { return ".json" }
func (p *jsonschemaPluginImpl) SubDir() string    { return "schemas" }

func (p *jsonschemaPluginImpl) Extract(contract *Contract) string {
	return contract.ExtractJSONSchema()
}

func (p *jsonschemaPluginImpl) Generate(spec string, sourceFile string) string {
	return generateContractFromSpec("schema", spec, sourceFile)
}

func (p *jsonschemaPluginImpl) Detect(path string, content []byte) bool {
	return detectJSONSchema(path, content)
}

// Helper functions for built-in plugins

func generateContractFromSpec(format string, specContent string, sourceFile string) string {
	basename := filepath.Base(sourceFile)
	nameWithoutExt := strings.TrimSuffix(basename, filepath.Ext(basename))
	contractID := strings.ToUpper(strings.ReplaceAll(nameWithoutExt, "-", "_"))
	contractID = strings.ReplaceAll(contractID, ".", "_")

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# CONTRACT-%s.1.0\n\n", contractID))
	md.WriteString(fmt.Sprintf("> Auto-generated from %s on %s\n\n", basename, time.Now().Format("2006-01-02")))

	switch format {
	case "gherkin":
		md.WriteString("## Purpose\n\n")
		md.WriteString(fmt.Sprintf("Behavior specification imported from %s.\n\n", basename))
		md.WriteString("## Scenarios\n\n")
		md.WriteString("```gherkin\n")
		md.WriteString(specContent)
		md.WriteString("\n```\n\n")

	case "mermaid":
		md.WriteString("## Purpose\n\n")
		md.WriteString(fmt.Sprintf("Architecture diagram imported from %s.\n\n", basename))
		md.WriteString("## Architecture\n\n")
		md.WriteString("```mermaid\n")
		md.WriteString(specContent)
		md.WriteString("\n```\n\n")

	case "openapi":
		md.WriteString("## Purpose\n\n")
		md.WriteString(fmt.Sprintf("API specification imported from %s.\n\n", basename))
		md.WriteString("## API\n\n")
		md.WriteString("```yaml\n")
		md.WriteString(specContent)
		md.WriteString("\n```\n\n")

	case "schema":
		md.WriteString("## Purpose\n\n")
		md.WriteString(fmt.Sprintf("Data schema imported from %s.\n\n", basename))
		md.WriteString("## Data Model\n\n")
		md.WriteString("```json\n")
		md.WriteString(specContent)
		md.WriteString("\n```\n\n")
	}

	md.WriteString("## Implementation\n\n")
	md.WriteString("_TODO: Document implementing files and components._\n\n")
	md.WriteString("## Testing\n\n")
	md.WriteString("_TODO: Document test strategy._\n")

	return md.String()
}

func detectGherkin(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".feature" {
		return true
	}
	text := string(content)
	return strings.Contains(text, "Feature:") && (strings.Contains(text, "Scenario:") || strings.Contains(text, "Given "))
}

func detectMermaid(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mmd" || ext == ".mermaid" {
		return true
	}
	text := strings.ToLower(string(content))
	return strings.Contains(text, "graph ") || strings.Contains(text, "sequenceDiagram") ||
		strings.Contains(text, "classDiagram") || strings.Contains(text, "erDiagram") ||
		strings.Contains(text, "flowchart ")
}

func detectOpenAPI(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "openapi:") || strings.Contains(text, "paths:")
}

func detectJSONSchema(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "$schema")
}
