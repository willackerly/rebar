package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/integrity"
)

var agentRole string

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Sealed envelope agent execution",
}

var agentStartCmd = &cobra.Command{
	Use:   "start [task description]",
	Short: "Launch an agent in an isolated worktree with role-based permissions",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAgentStart,
}

var agentFinishCmd = &cobra.Command{
	Use:   "finish [envelope-id]",
	Short: "Audit agent work and report integrity status",
	RunE:  runAgentFinish,
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active agent envelopes",
	RunE:  runAgentList,
}

func init() {
	agentStartCmd.Flags().StringVar(&agentRole, "role", "developer", "agent role (developer, tester, architect, steward)")
	agentCmd.AddCommand(agentStartCmd)
	agentCmd.AddCommand(agentFinishCmd)
	agentCmd.AddCommand(agentListCmd)
}

// Envelope is the sealed state for an agent execution.
type Envelope struct {
	ID           string    `json:"id"`
	Role         string    `json:"role"`
	Task         string    `json:"task"`
	WorktreeDir  string    `json:"worktree_dir"`
	BranchName   string    `json:"branch_name"`
	BaseCommit   string    `json:"base_commit"`
	StartedAt    time.Time `json:"started_at"`
	SnapshotPath string    `json:"manifest_snapshot"`
}

// RolePermissions defines writable/read-only glob patterns per role.
var rolePermissions = map[string]struct {
	Writable []string
	ReadOnly []string
}{
	"developer": {
		Writable: []string{"src/", "internal/", "cmd/", "client/", "packages/", "lib/", "app/"},
		ReadOnly: []string{"tests/", "test/", "scripts/", "architecture/"},
	},
	"tester": {
		Writable: []string{"tests/", "test/"},
		ReadOnly: []string{"src/", "internal/", "scripts/", "architecture/"},
	},
	"architect": {
		Writable: []string{"architecture/"},
		ReadOnly: []string{"src/", "tests/", "scripts/"},
	},
	"steward": {
		Writable: []string{},
		ReadOnly: []string{"src/", "tests/", "scripts/", "architecture/"},
	},
}

func runAgentStart(cmd *cobra.Command, args []string) error {
	task := strings.Join(args, " ")
	id := uuid.New().String()[:8]

	// Get current HEAD
	baseCommit := gitOutput("rev-parse", "HEAD")
	if baseCommit == "" {
		return fmt.Errorf("could not determine HEAD commit")
	}

	// Create worktree
	branchName := fmt.Sprintf("agent/%s-%s", agentRole, id)
	worktreeDir := filepath.Join(filepath.Dir(cfg.RepoRoot), filepath.Base(cfg.RepoRoot)+"-worktrees", branchName)

	wtCmd := exec.Command("git", "-C", cfg.RepoRoot, "worktree", "add", "-b", branchName, worktreeDir)
	if out, err := wtCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating worktree: %s", string(out))
	}

	// Snapshot manifest
	snapshotPath := filepath.Join(cfg.RebarDir, "envelopes", id+"-snapshot.json")
	manifest, err := integrity.LoadManifest(cfg.RebarDir)
	if err == nil {
		data, _ := json.MarshalIndent(manifest, "", "  ")
		os.WriteFile(snapshotPath, data, 0644)
	}

	// Save envelope
	envelope := Envelope{
		ID:           id,
		Role:         agentRole,
		Task:         task,
		WorktreeDir:  worktreeDir,
		BranchName:   branchName,
		BaseCommit:   baseCommit,
		StartedAt:    time.Now().UTC(),
		SnapshotPath: snapshotPath,
	}

	envPath := filepath.Join(cfg.RebarDir, "envelopes", id+".json")
	data, _ := json.MarshalIndent(envelope, "", "  ")
	if err := os.WriteFile(envPath, data, 0644); err != nil {
		return fmt.Errorf("saving envelope: %w", err)
	}

	// Report
	fmt.Printf("\nAgent Envelope Created\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  ID:         %s\n", id)
	fmt.Printf("  Role:       %s\n", agentRole)
	fmt.Printf("  Task:       %s\n", task)
	fmt.Printf("  Branch:     %s\n", branchName)
	fmt.Printf("  Worktree:   %s\n", worktreeDir)
	fmt.Printf("  Base:       %s\n", baseCommit[:12])

	perms, ok := rolePermissions[agentRole]
	if ok {
		if len(perms.Writable) > 0 {
			fmt.Printf("  Writable:   %s\n", strings.Join(perms.Writable, ", "))
		} else {
			fmt.Printf("  Writable:   (none — read-only role)\n")
		}
		fmt.Printf("  Read-only:  %s\n", strings.Join(perms.ReadOnly, ", "))
	}

	fmt.Printf("\nTo work in this worktree:\n")
	fmt.Printf("  cd %s\n", worktreeDir)
	fmt.Printf("\nWhen done:\n")
	fmt.Printf("  rebar agent finish %s\n", id)

	return nil
}

func runAgentFinish(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("envelope ID required — see 'rebar agent list'")
	}

	id := args[0]
	envPath := filepath.Join(cfg.RebarDir, "envelopes", id+".json")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("envelope %s not found", id)
	}

	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("parsing envelope: %w", err)
	}

	fmt.Printf("Agent Audit — %s task: %q\n", envelope.Role, envelope.Task)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Get changed files in worktree
	diffCmd := exec.Command("git", "-C", envelope.WorktreeDir, "diff", "--name-only", envelope.BaseCommit+"..HEAD")
	diffOut, err := diffCmd.Output()
	if err != nil {
		return fmt.Errorf("diffing worktree: %w", err)
	}

	changedFiles := strings.Split(strings.TrimSpace(string(diffOut)), "\n")
	if len(changedFiles) == 1 && changedFiles[0] == "" {
		changedFiles = nil
	}

	if len(changedFiles) == 0 {
		fmt.Println("\nNo changes detected in worktree.")
		return nil
	}

	// Check permissions
	perms, hasPerms := rolePermissions[envelope.Role]
	violations := []string{}
	allowed := []string{}

	for _, file := range changedFiles {
		permitted := false
		if hasPerms {
			for _, w := range perms.Writable {
				if strings.HasPrefix(file, w) {
					permitted = true
					break
				}
			}
		}

		if permitted || !hasPerms {
			allowed = append(allowed, file)
		} else {
			violations = append(violations, file)
		}
	}

	if len(violations) > 0 {
		fmt.Println("\nVIOLATIONS:")
		for _, v := range violations {
			fmt.Printf("  ✗ %s was MODIFIED (role: %s, not in writable paths)\n", v, envelope.Role)
		}
	}

	if len(allowed) > 0 {
		fmt.Println("\nALLOWED CHANGES:")
		for _, a := range allowed {
			fmt.Printf("  ✓ %s — role: %s\n", a, envelope.Role)
		}
	}

	// Verify integrity in worktree
	wtRebarDir := filepath.Join(envelope.WorktreeDir, ".rebar")
	if manifest, err := integrity.LoadManifest(wtRebarDir); err == nil {
		salt, _ := integrity.LoadSalt(wtRebarDir)
		if result, err := integrity.Verify(envelope.WorktreeDir, manifest, salt); err == nil {
			for _, r := range result.Ratchets {
				if r.Violated {
					fmt.Printf("\n  ✗ Ratchet violation: %s is %d, min is %d\n", r.Name, r.Current, r.Min)
					violations = append(violations, "ratchet:"+r.Name)
				}
			}
		}
	}

	// Summary
	if len(violations) > 0 {
		fmt.Printf("\nACTION: Agent work quarantined. Review violations before merging.\n")
		fmt.Printf("  Worktree: %s\n", envelope.WorktreeDir)
		fmt.Printf("  Branch:   %s\n", envelope.BranchName)
	} else {
		fmt.Printf("\nRESULT: All changes within role permissions ✓\n")
		fmt.Printf("  To merge: git merge %s\n", envelope.BranchName)
		fmt.Printf("  To clean: git worktree remove %s\n", envelope.WorktreeDir)
	}

	return nil
}

func runAgentList(cmd *cobra.Command, args []string) error {
	envelopesDir := filepath.Join(cfg.RebarDir, "envelopes")
	entries, err := os.ReadDir(envelopesDir)
	if err != nil {
		fmt.Println("No active envelopes.")
		return nil
	}

	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") && !strings.Contains(e.Name(), "snapshot") {
			data, err := os.ReadFile(filepath.Join(envelopesDir, e.Name()))
			if err != nil {
				continue
			}
			var env Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			if !found {
				fmt.Println("Active Envelopes:")
				fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
				found = true
			}
			age := time.Since(env.StartedAt).Round(time.Minute)
			fmt.Printf("  %s  role: %-10s  age: %-10s  task: %s\n", env.ID, env.Role, age, env.Task)
		}
	}

	if !found {
		fmt.Println("No active envelopes.")
	}

	return nil
}
