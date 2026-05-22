package spec

import "time"

// ExportOptions configures contract export to standard formats
type ExportOptions struct {
	RepoRoot    string   // Repository root path
	ContractDir string   // Directory containing contracts (default: architecture)
	OutDir      string   // Output directory for specs (default: specs)
	Format      string   // Export only this format (empty = all)
	Force       bool     // Overwrite existing files
	DryRun      bool     // Show what would be exported
	Patterns    []string // Contract file patterns (empty = all)
}

// ImportOptions configures spec import to REBAR contracts
type ImportOptions struct {
	RepoRoot    string   // Repository root path
	ContractDir string   // Directory for contracts (default: architecture)
	SpecPaths   []string // Paths to spec files to import
	Force       bool     // Overwrite existing contracts
	DryRun      bool     // Show what would be imported
}

// SyncOptions configures bidirectional sync
type SyncOptions struct {
	RepoRoot    string // Repository root path
	ContractDir string // Directory containing contracts
	OutDir      string // Spec output directory
	Force       bool   // Resolve conflicts by overwriting
	DryRun      bool   // Show changes without applying
}

// Manifest tracks sync state between contracts and specs
type Manifest struct {
	Version  string        `json:"version"`
	LastSync time.Time     `json:"lastSync"`
	Mappings []SpecMapping `json:"mappings"`
}

// SpecMapping tracks a single contract and its exported specs
type SpecMapping struct {
	Contract         string       `json:"contract"`         // Contract file path
	ContractChecksum string       `json:"contractChecksum"` // SHA256 of contract
	Exports          []ExportedSpec `json:"exports"`        // Exported spec files
}

// ExportedSpec represents one exported spec file
type ExportedSpec struct {
	Type     string `json:"type"`     // gherkin, mermaid, openapi, schema, adr
	Path     string `json:"path"`     // Spec file path relative to repo root
	Checksum string `json:"checksum"` // SHA256 of spec file
}

// Contract represents parsed contract metadata
type Contract struct {
	Path        string
	ID          string
	Name        string
	Version     string
	Content     string
	Sections    map[string]string // section name → content
	CodeBlocks  []CodeBlock
}

// CodeBlock represents a fenced code block in markdown
type CodeBlock struct {
	Language string
	Content  string
	Line     int
}

// SpecFormat represents a standard specification format
type SpecFormat string

const (
	FormatGherkin SpecFormat = "gherkin"
	FormatMermaid SpecFormat = "mermaid"
	FormatOpenAPI SpecFormat = "openapi"
	FormatSchema  SpecFormat = "schema"
	FormatADR     SpecFormat = "adr"
)

// ExportResult tracks the outcome of an export operation
type ExportResult struct {
	Contract string
	Exported []ExportedSpec
	Skipped  []string // Reasons for skipping
	Errors   []string
}

// ImportResult tracks the outcome of an import operation
type ImportResult struct {
	SpecPath string
	Contract string // Generated/updated contract path
	Created  bool   // true if new contract, false if updated
	Skipped  string // Reason for skipping
	Error    string
}

// SyncResult tracks the outcome of a sync operation
type SyncResult struct {
	ContractsExported int
	SpecsImported     int
	Conflicts         []SyncConflict
	Errors            []string
}

// SyncConflict represents a bidirectional change conflict
type SyncConflict struct {
	Contract string
	Spec     string
	Reason   string // "both-modified", "checksum-mismatch"
}
