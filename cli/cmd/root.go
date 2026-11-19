package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/willackerly/rebar/cli/internal/config"
)

// releaseVersion is the release string of this source tree. Bump it with
// each release; the release workflow refuses a tag that disagrees with it.
const releaseVersion = "v4.0.0-rc.1"

// Version is what `rebar version` reports. Release builds and setup-rebar.sh
// source builds stamp it with
//
//	go build -ldflags "-X github.com/willackerly/rebar/cli/cmd.Version=<ver>"
//
// (a tag, or <branch>@<sha7> for a branch build). Unstamped, it is
// releaseVersion.
var Version = releaseVersion

// semverTag matches the release tags check-compliance.sh accepts in a badge.
var semverTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

// badgeVersion is the version new/adopt write into a project's README badge,
// which check-compliance.sh validates: the stamped Version when it is a
// release tag, else this tree's releaseVersion, so a branch build does not
// write "feat/x@abc1234" into an adopter's badge.
func badgeVersion() string {
	if semverTag.MatchString(Version) {
		return Version
	}
	return releaseVersion
}

var (
	verbose  bool
	jsonOut  bool
	repoRoot string
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:     "rebar",
	Version: Version,
	Short:   "REBAR — contract-driven development framework for AI-powered teams",
	Long: `REBAR — contract-driven development framework for AI-powered teams.

  Get started:
    rebar new my-project -d "description"   Create a new REBAR project
    rebar adopt                              Add REBAR to an existing project
    rebar init                               Initialize integrity tracking

  Daily workflow:
    rebar context [role]                     View context files for a role
    rebar commit -m "message"                Enforced commit with integrity
    rebar audit                              Check compliance score
    rebar audit --all ~/dev                  Fleet-wide scorecard

  Agent coordination:
    rebar ask <agent> "question"             Query a role-based agent
    rebar agent start --role dev "task"       Run agent in sealed envelope

  Quality & integrity:
    rebar status                             Health dashboard
    rebar verify                             Check file integrity
    rebar check                              Run all enforcement checks
    rebar run <script>                       Run one of rebar's scripts
    rebar migrate [--apply]                  Move legacy script copies into .rebarrc`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip repo detection for commands that handle their own root
		name := cmd.Name()
		if name == "help" || name == "version" || name == "native" || name == "install-shims" {
			return nil // native finds its own root (nativeRoot)
		}
		// ASK works in any directory, as bin/ask always did.
		if (name == "ask" || name == "ask-agent-loop") && cmd.Parent() != nil && cmd.Parent().Name() == "rebar" {
			return nil
		}
		if p := cmd.Parent(); p != nil && p.Name() == "mcp" && p.Parent() != nil && p.Parent().Name() == "rebar" {
			return nil // the MCP server and client serve whatever repos they are given
		}
		// These commands handle their own repo root
		if name == "init" || name == "new" || name == "audit" || name == "adopt" {
			if cmd.Parent() != nil && cmd.Parent().Name() == "rebar" {
				return nil
			}
		}

		var err error
		if repoRoot == "" {
			// Like the scripts: REBAR_PROJECT_ROOT names the project (the
			// thin wrappers `rebar migrate` leaves set it, so a copied
			// wrapper still checks the directory it sits in).
			repoRoot = os.Getenv("REBAR_PROJECT_ROOT")
		}
		if repoRoot == "" {
			cwd, _ := os.Getwd()
			repoRoot, err = config.FindRepoRoot(cwd)
			if err != nil {
				return fmt.Errorf("not a rebar repository: %w\nRun 'rebar init' to initialize or 'rebar new <name>' to create a project", err)
			}
		}

		cfg, err = config.Load(repoRoot)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI. Cobra's own error printing is off (SilenceErrors),
// so the error is printed here, once, on stderr; without this a failing
// command (e.g. "rebar install not found (set REBAR_ROOT …)") exited 1 silently.
func Execute() error {
	return executeArgs(os.Args[1:])
}

// executeArgs runs the CLI on args (os.Args[1:] outside tests).
func executeArgs(args []string) error {
	args, err := hoistRootFlags(args)
	if err == nil {
		rootCmd.SetArgs(args)
		err = rootCmd.Execute()
	}
	if err != nil {
		fmt.Fprintln(rootCmd.ErrOrStderr(), "rebar:", err)
	}
	return err
}

// rootFlagsAfterName marks a pass-through command whose first argument can
// never start with "-" (run's script name), so root flags right after the
// command name are hoisted too: `rebar run -v steward`.
const rootFlagsAfterName = "rebar/root-flags-after-name"

// hoistRootFlags parses root persistent flags (--repo-root, -v, --json) that
// precede a pass-through command and drops them from args. Pass-through
// commands (run, ask, push) set DisableFlagParsing so their arguments reach
// the script or tool untouched, and for such a command cobra hands over every
// argument but the command name, root flags included: `rebar --repo-root X
// run steward` reached run as ["--repo-root" "X" "steward"]. Only flags before
// the command name are taken (plus, for rootFlagsAfterName commands, right
// after it); anything else, including flags after the script name, passes
// through. args come back unchanged when they do not start with root flags
// followed by a pass-through command, and cobra handles them as before.
func hoistRootFlags(args []string) ([]string, error) {
	n, clean := rootFlagPrefix(args)
	if !clean || n >= len(args) {
		return args, nil
	}
	sub := passThroughCommand(args[n])
	if sub == nil {
		return args, nil
	}
	flags := append([]string{}, args[:n]...)
	rest := args[n+1:]
	if sub.Annotations[rootFlagsAfterName] == "true" {
		m, _ := rootFlagPrefix(rest)
		flags = append(flags, rest[:m]...)
		rest = rest[m:]
	}
	if err := rootCmd.PersistentFlags().Parse(flags); err != nil {
		return nil, err
	}
	return append([]string{args[n]}, rest...), nil
}

// rootFlagPrefix counts the leading args that are root persistent flags,
// with their values. clean is false when it stopped at a flag the root does
// not define.
func rootFlagPrefix(args []string) (n int, clean bool) {
	flags := rootCmd.PersistentFlags()
	for n < len(args) {
		a := args[n]
		if a == "--" || len(a) < 2 || a[0] != '-' {
			return n, true
		}
		var f *pflag.Flag
		name, _, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if strings.HasPrefix(a, "--") {
			f = flags.Lookup(name)
		} else if len(name) == 1 {
			f = flags.ShorthandLookup(name)
		}
		if f == nil {
			return n, false
		}
		n++
		if !hasValue && f.NoOptDefVal == "" {
			if n >= len(args) {
				return n - 1, false // value missing: let cobra report it
			}
			n++
		}
	}
	return n, true
}

// passThroughCommand returns the root subcommand called name when it takes
// its arguments verbatim (DisableFlagParsing), else nil.
func passThroughCommand(name string) *cobra.Command {
	for _, c := range rootCmd.Commands() {
		if (c.Name() == name || c.HasAlias(name)) && c.DisableFlagParsing {
			return c
		}
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "machine-readable JSON output")
	rootCmd.PersistentFlags().StringVar(&repoRoot, "repo-root", "", "override repo root detection")

	// Group commands by purpose using cobra groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "start", Title: "Getting Started:"},
		&cobra.Group{ID: "daily", Title: "Daily Workflow:"},
		&cobra.Group{ID: "agents", Title: "Agent Coordination:"},
		&cobra.Group{ID: "quality", Title: "Quality & Integrity:"},
		&cobra.Group{ID: "keys", Title: "Signing & Keys:"},
	)

	// Getting Started
	newCmd.GroupID = "start"
	adoptCmd.GroupID = "start"
	initCmd.GroupID = "start"
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(initCmd)
	installShimsCmd.GroupID = "start"
	rootCmd.AddCommand(installShimsCmd)

	// Daily Workflow
	contextCmd.GroupID = "daily"
	commitCmd.GroupID = "daily"
	auditCmd.GroupID = "daily"
	pushCmd.GroupID = "daily"
	docCmd.GroupID = "daily"
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(docCmd)

	// Agent Coordination
	askCmd.GroupID = "agents"
	agentCmd.GroupID = "agents"
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(agentCmd)

	// Quality & Integrity
	statusCmd.GroupID = "quality"
	verifyCmd.GroupID = "quality"
	checkCmd.GroupID = "quality"
	runCmd.GroupID = "quality"
	diffCmd.GroupID = "quality"
	contractCmd.GroupID = "quality"
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(contractCmd)

	// Signing & Keys
	signCmd.GroupID = "keys"
	keyCmd.GroupID = "keys"
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(keyCmd)

	// Hidden, experimental: native check implementations (native.go).
	rootCmd.AddCommand(nativeCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print rebar version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("rebar " + rootCmd.Version)
		},
	})
}
