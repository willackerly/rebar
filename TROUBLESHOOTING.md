# Troubleshooting Guide

**Common issues and solutions for rebar adoption**

This guide covers the most frequent problems teams encounter when adopting rebar, with specific solutions and workarounds.

---

## 🚨 Setup & Getting Started

### "I'm overwhelmed by all the documentation"
**Symptoms:** Too many files, don't know where to start, analysis paralysis

**Solution:**
1. **Ignore everything except README.md and QUICKSTART.md**
2. Follow only the 5-minute solo setup
3. Write one simple contract and link it to code
4. Stop there until it feels natural
5. Then gradually explore [FEATURE-DEVELOPMENT.md](FEATURE-DEVELOPMENT.md)

**Remember:** You don't need to understand the whole system to get value from it.

### "Setup doesn't work for my mature project"
**Symptoms:** Templates don't fit, existing docs better than templates, wholesale replacement feels wrong

**Solution:**
- **Don't replace existing docs** — merge rebar's gaps in
- Focus on highest-value additions: contract headers, enforcement scripts, agent roles
- See [blindpipe case study](feedback/processed/blindpipe-adoption-2026-03-19.md) for selective adoption patterns
- Consider tier 1 (partial) adoption first

### "Scripts fail with permission errors"
**Symptoms:** `./scripts/check-*.sh` fails with "Permission denied"

**Solution:**
```bash
chmod +x scripts/*.sh
```

### "check-contract-refs.sh fails"
**Symptoms:** Script reports "CONTRACT: ref doesn't match any contract file"

**Common causes:**
- **Filename mismatch:** `CONTRACT:C1-BLOBSTORE.1.0` but file is `CONTRACT-C1-BLOBSTORE.1.1.md`
- **Version mismatch:** Contract updated but code references old version
- **Typo in header:** `CONTRAACT:` or `CONTRACT C1` (missing colon)

**Solution:**
```bash
# Find all contract references
grep -rn "CONTRACT:" src/

# Find all contract files
ls architecture/CONTRACT-*.md

# Make sure they match exactly
```

---

## 🤝 Agent Coordination

### "Agents aren't giving helpful answers"
**Symptoms:** Generic responses, agents claim not to know things they should know, responses don't match your codebase

**Solutions:**
1. **Update the Cold Start Quad:**
   - Make sure `QUICKCONTEXT.md` reflects current project state
   - Keep `TODO.md` current with active work
   - Verify `AGENTS.md` matches your actual workflow

2. **Reset agent sessions:**
   ```bash
   ask reset architect  # Clear stale context
   ask reset product    # Start fresh
   ```

3. **Check file structure:**
   ```bash
   ask who  # Should list: architect, englead, merger, product, steward, tester
   ```

### "Don't understand role agents vs subagent templates"
**Symptoms:** Confusion about when to use `ask architect` vs `claude --prompt agents/subagent-prompts/code-review.md`

**Solution:**
- **Questions & guidance** → Role agents (`ask architect "should I..."`)
- **Focused work tasks** → Subagent templates (`claude --prompt ...`)
- See [agents/README.md](agents/README.md) for the decision tree and the 6 core roles

### "Multi-agent coordination creates conflicts"
**Symptoms:** Merge conflicts, agents stepping on each other's work, lost changes

**Solutions:**
1. **Use worktree isolation** — each agent gets its own working copy
2. **Follow practices/worktree-collaboration.md** for coordination patterns
3. **See [OpenDocKit case study](feedback/processed/2026-03-18-opendockit-fidelity-session.md)** for 9-agent coordination patterns
4. **Consider [Human-based Digital Signer approach](feedback/processed/digital-signer-feedback.md)** — 18 agents, 0 conflicts

---

## 📋 Contracts & Documentation

### "Contract system seems too complex"
**Symptoms:** Don't understand behavioral contracts vs interfaces, versioning seems heavyweight

**Start simple approach:**
1. **Use [architecture/CONTRACT-TEMPLATE.md](architecture/CONTRACT-TEMPLATE.md)** — annotated template, all sections explained inline
2. **Start with obvious behaviors:** What happens on missing key? Empty input? Error cases?
3. **Don't worry about versioning initially** — use 1.0 for everything
4. **See [FEATURE-DEVELOPMENT.md](FEATURE-DEVELOPMENT.md)** auth example for concrete pattern

### "Numbers in docs drift from reality"
**Symptoms:** Docs say "126 tests" but you have 586, test counts wrong, metrics stale

**Solution:**
- Implement **ground truth enforcement** from [Human-based Digital Signer case](feedback/processed/digital-signer-feedback.md)
- Use `METRICS.template` file with computed values
- Set up `scripts/check-ground-truth.sh` to verify claims
- Consider tier 2 or 3 for automated enforcement

### "Documentation freshness is hard to maintain"
**Symptoms:** Stale dates, outdated examples, docs lag behind code changes

**Solutions:**
1. **Use freshness enforcement:**
   ```bash
   scripts/check-freshness.sh  # Catches stale dates
   ```

2. **Touch files when you modify them:**
   ```bash
   # Update freshness date in file header
   <!-- freshness: YYYY-MM-DD -->
   ```

3. **Consider agent-driven updates** — agents can update docs as they work

---

## ⚡ Quality & Testing

### "Quality enforcement too strict/too loose"
**Symptoms:** CI failing on minor issues OR quality problems slipping through

**Solution — Adjust your tier:**
```bash
# Edit .rebarrc
REBAR_TIER=1  # Partial: just contract refs + TODOs
REBAR_TIER=2  # Adopted: + headers, freshness, registry
REBAR_TIER=3  # Enforced: + ground truth, strict steward
```

**Guidance:**
- **Solo dev, new to rebar:** Start with tier 1
- **Small team, established workflow:** Use tier 2
- **Department, mission-critical:** Use tier 3
- See [scalability assessment](feedback/processed/scalability-assessment-2026-03-20.md) for progression

### "Testing cascade seems overwhelming"
**Symptoms:** T0-T5 feels like too many testing levels, unclear which tests to write

**Practical approach:**
1. **Start with T0-T1:** Unit tests + basic integration
2. **Add T2 for security-critical code:** Auth, crypto, input validation
3. **T3+ for production systems:** See [zero-tolerance testing](feedback/processed/zero-tolerance-testing-feedback.md)
4. **Follow [FEATURE-DEVELOPMENT.md](FEATURE-DEVELOPMENT.md)** example for progression

### "CI takes too long with all the checks"
**Symptoms:** Pre-commit hooks slow, CI pipeline times out, too many quality gates

**Solutions:**
1. **Tier down temporarily** during active development
2. **Run expensive checks nightly,** fast checks on PR
3. **Parallel execution** of independent checks
4. **See [practices/multi-agent-orchestration.md](practices/multi-agent-orchestration.md)** for parallelization

---

## 🔧 Technical Issues

### "ASK CLI not working"
**Symptoms:** `ask who` shows no agents, commands fail, can't find agents

**Solutions:**
1. **Check directory structure:**
   ```bash
   ls agents/        # Should have: architect/, englead/, etc.
   ask where architect  # Shows path resolution
   ```

2. **Verify agent files:**
   ```bash
   ls agents/architect/AGENT.md  # Should exist
   ```

3. **Check environment:**
   ```bash
   echo $ASK_AGENTS_DIR  # Should be ./agents or custom path
   ```

### "Cross-project queries not working"
**Symptoms:** `ask project:agent "question"` fails, multi-repo coordination broken

**Solutions:**
1. **Check ASK_SERVER setup:**
   ```bash
   echo $ASK_SERVER  # Should be host:port
   ```

2. **Register projects:**
   ```bash
   ask register myproject  # From project directory
   ask projects            # List all registered
   ```

3. **Test connectivity:**
   ```bash
   ask myproject:architect "test question"
   ```

### "Version compatibility issues"
**Symptoms:** Templates don't work, scripts fail, features missing

**Solutions:**
1. **Check rebar version:**
   ```bash
   cat .rebar-version  # Should match your rebar checkout
   ```

2. **See migration guide:**
   - [versioning-and-upgrade-path](feedback/processed/versioning-and-upgrade-path-2026-03-20.md)
   - Follow backwards compatibility patterns

---

## 🎯 Scaling & Adoption

### "Team members resist adoption"
**Symptoms:** "This is too complex", "We don't need this", "Our current docs work fine"

**Solutions:**
1. **Start with value demonstration:**
   - Show agent coordination benefits immediately
   - Use [QUICKSTART.md](QUICKSTART.md) 5-minute demo
   - Focus on "this prevents bugs" not "this is documentation"

2. **Gradual adoption:**
   - Begin with contract headers only
   - Add quality scripts once headers are natural
   - Introduce agents after team sees value

3. **Show real results:**
   - [Human-based Digital Signer](feedback/processed/digital-signer-feedback.md): 0 merge conflicts
   - [blindpipe](feedback/processed/blindpipe-adoption-2026-03-19.md): 10x context efficiency
   - [OpenDocKit](feedback/processed/2026-03-18-opendockit-fidelity-session.md): 100% work recovery

### "Scaling beyond small team"
**Symptoms:** Single-repo patterns don't work for multiple teams, cross-repo dependencies unclear

**Solutions:**
1. **Follow tier progression:**
   - [profiles/solo-dev.md](profiles/solo-dev.md) → [profiles/small-team.md](profiles/small-team.md) → [profiles/department.md](profiles/department.md)

2. **Implement cross-repo patterns:**
   - Contract namespacing: `CONTRACT:project/C1-BLOBSTORE.2.1`
   - Shared contract catalogs
   - Breaking change detection across repos

3. **Study enterprise patterns:**
   - [Scalability Assessment](feedback/processed/scalability-assessment-2026-03-20.md) for 1000-dev organization

---

## 📞 Getting Help

### Quick Self-Help
1. **Check [README.md](README.md)** for the navigation gateway
2. **Search [CASE-STUDIES.md](CASE-STUDIES.md)** for similar problems
3. **Ask your agents:** `ask steward "what issues exist?"` or `ask architect "review my setup"`

### Escalation Path
1. **File-specific issues:** Check template comments and examples
2. **Workflow questions:** Review [practices/](practices/) guides
3. **Complex coordination:** Study [feedback/](feedback/) case studies
4. **Agent behavior:** Try `ask reset <agent>` and update Cold Start Quad

**Remember:** Most issues are configuration, not fundamental problems. The patterns in this guide have been proven across 100+ agent launches in production systems.