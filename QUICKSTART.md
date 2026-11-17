📍 **You are here:** **Try It (5 min)** → [Love It (1 hour)](FEATURE-DEVELOPMENT.md) → [Master It](CASE-STUDIES.md)

# Rebar Quickstart

**Get rebar working in 5 minutes**

Experience rebar's value immediately: write one contract, link it to code, see agent coordination in action.

---

## Solo Developer Setup

### Step 1: Install rebar and create a project (2 minutes)
```bash
# Install the rebar CLI (versions side by side under ~/.rebar/versions/)
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.1/setup-rebar.sh | bash -s -- v3.1.1
# Open a new terminal (or source your shell RC) so PATH picks up ~/.rebar/current/bin
rebar version

# Create a project: git repo, scaffolding, role agents, first commit
rebar new my-new-project
cd my-new-project

# Enforce the checks on every commit
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit
```

Trying a release candidate? Install it from its own tag:
[SETUP.md § Testing a release candidate](SETUP.md#testing-a-release-candidate).
Adding rebar to an existing repository instead? Run `rebar adopt --tier 1`
in it, then link the hook the same way. Already using an older rebar? See
[UPGRADING.md](UPGRADING.md).

> **What `rebar new` sets up:** the Cold Start Quad (README, QUICKCONTEXT,
> TODO, AGENTS), `CLAUDE.md`, `architecture/` with the contract template,
> the six ASK role agents (architect, product, englead, steward, merger,
> featurerequest), `.mcp.json`, a Claude Code SessionStart hook, and
> `scripts/ci-check.sh` + `scripts/pre-commit.sh`. Those two only call
> `rebar check`: the checks themselves run from your rebar install, and the
> project tailors them in `.rebarrc` ([every setting](docs/CONFIGURATION.md)). `-d "what it does"` additionally asks an
> LLM to draft the README and first contracts.

### Step 2: Write your first contract (3 minutes)

**Create a simple file service contract:**
```bash
# Create your first contract
touch architecture/CONTRACT-C1-FILESTORE.1.0.md
```

```markdown
# CONTRACT-C1-FILESTORE.1.0

**Version:** 1.0
**Status:** draft
**Owner:** you

## Purpose
Simple file storage for user uploads. Save/retrieve files by key.

## Interface
```go
type FileStore interface {
    Save(key string, data []byte) error
    Load(key string) ([]byte, error)
    Delete(key string) error
}
```

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| Save with empty data | Returns ErrInvalidInput |
| Load missing file | Returns ErrNotFound |
| Delete missing file | No-op, returns nil |
```

**Link it to code:**
```go
// filestore.go
// CONTRACT:C1-FILESTORE.1.0
package main

func Save(key string, data []byte) error {
    if len(data) == 0 {
        return ErrInvalidInput  // Contract compliance
    }
    // ... implementation
}
```

**Verify the link and commit:**
```bash
grep -r "CONTRACT:C1-FILESTORE" .
rebar run check-contract-refs   # Should pass
git add -A                      # the registry counts implementing files git tracks
rebar run compute-registry      # add the contract to architecture/CONTRACT-REGISTRY.md
rebar check                     # the full suite CI runs
git add -A && git commit -m "feat: first contract"   # the hook runs the pre-commit checks
```

---

## You're Done! 🎉

**What you just experienced:**
- ✅ **Information organization** — contract linked to implementation
- ✅ **Quality automation** — verification scripts catch drift
- ✅ **Agent coordination** — any agent can now implement this contract correctly

**Test agent coordination:**
```bash
rebar ask architect "Review CONTRACT:C1-FILESTORE.1.0 - any integration concerns?"
rebar ask product "Does this file storage meet basic user needs?"
```

**Wire ASK into Claude Code (MCP):**
`rebar init` already wrote `.mcp.json` to your project root. Reload
Claude Code in this directory and the `ask_<repo>_<role>` tools appear
in the tool list automatically — no more shelling out.
**[→ MCP Setup Guide](docs/MCP-SETUP.md)** if you want user-level or
multi-repo setups.

**Check integrity:**
```bash
rebar init       # Initialize integrity tracking + MCP wiring
rebar verify     # Verify all protected files are clean
rebar status     # Quick health dashboard
```

---

## 🎯 **Next Steps**

### Ready for the full experience?
**→ [FEATURE-DEVELOPMENT.md](FEATURE-DEVELOPMENT.md)** — Complete BDD → Contract → Code → Test workflow (1 hour)

### Want to understand agent coordination?
**→ [agents/README.md](agents/README.md)** — Role agents vs subagent templates, decision tree, the 6 core roles

### Need to solve a specific problem?
**→ [CASE-STUDIES.md](CASE-STUDIES.md)** — Real-world solutions indexed by problem type

### Ready to scale your team?
**→ [profiles/](profiles/)** — Solo dev → small team → department progression

---

## 🆘 **Troubleshooting**

| Problem | Solution |
|---------|----------|
| `rebar: command not found` | Open a new terminal or source your shell RC; the installer added `~/.rebar/current/bin` to it |
| `rebar: ci-check.sh not found in the project or a rebar install` | The CLI found no rebar install to run checks from. Check `rebar version` and unset a stale `REBAR_ROOT`; lookup order: [SETUP.md § Run rebar in CI](SETUP.md#run-rebar-in-ci) |
| Registry check fails after adding a contract | `rebar run compute-registry`, then commit the registry |
| Contract check fails | Make sure CONTRACT: comment matches filename exactly |
| Agents give generic answers | Fill in QUICKCONTEXT.md with your project details |
| Too overwhelming | Stick to just this quickstart until it feels natural |

**Remember:** Start small, build confidence, then expand. The 5-minute setup you just did is the foundation for everything else rebar offers.