📍 **You are here:** **Try It (5 min)** → [Love It (1 hour)](FEATURE-DEVELOPMENT.md) → [Master It](CASE-STUDIES.md)

# Rebar Quickstart

**Get rebar working in 5 minutes**

Experience rebar's value immediately: write one contract, link it to code, see agent coordination in action.

---

## Solo Developer Setup

### Step 1: Copy the bootstrap template (30 seconds)
```bash
# Clone rebar
git clone https://github.com/willackerly/rebar.git
cd rebar

# Copy complete working project template
cp -r templates/project-bootstrap/* ../my-new-project/
cd ../my-new-project

# Install rebar CLI tools + agent access
../rebar/bin/install --server 192.168.0.181:7232
```

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

**Verify the link:**
```bash
grep -r "CONTRACT:C1-FILESTORE" .
scripts/check-contract-refs.sh  # Should pass
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
**→ [AGENTS-QUICKSTART.md](AGENTS-QUICKSTART.md)** — Role agents vs subagent templates (15 min)

### Need to solve a specific problem?
**→ [CASE-STUDIES.md](CASE-STUDIES.md)** — Real-world solutions indexed by problem type

### Ready to scale your team?
**→ [profiles/](profiles/)** — Solo dev → small team → department progression

---

## 🆘 **Troubleshooting**

| Problem | Solution |
|---------|----------|
| Scripts fail with permissions | `chmod +x scripts/*.sh` |
| Contract check fails | Make sure CONTRACT: comment matches filename exactly |
| Agents give generic answers | Fill in QUICKCONTEXT.md with your project details |
| Too overwhelming | Stick to just this quickstart until it feels natural |

**Remember:** Start small, build confidence, then expand. The 5-minute setup you just did is the foundation for everything else rebar offers.