# Rebar Templates

**Ready-to-use templates for bootstrapping new projects and components.**

---

## Quick Bootstrap

### Complete Project Template
```bash
# From within the rebar checkout
cp -r templates/project-bootstrap/* ../my-new-project/
cd ../my-new-project

# Install CLI tools and configure agents
../rebar/bin/install --server your-ask-server:port
```

**What you get:** Working rebar project with filled examples, ready to run immediately.

### Individual Components
```bash
# Copy specific template files
cp templates/component-templates/AGENTS.template.md AGENTS.md
cp templates/component-templates/CLAUDE.template.md CLAUDE.md
cp architecture/CONTRACT-TEMPLATE.md architecture/CONTRACT-C1-MYSERVICE.1.0.md
```

---

## Templates Available

### 📁 **project-bootstrap/**
*Complete starter project — copy this to get running immediately*

**Contents:**
- **README.md** — Project documentation with rebar integration
- **QUICKCONTEXT.md** — Current state template with realistic examples
- **TODO.md** — Task tracking with starter tasks
- **AGENTS.md** — Agent coordination guidelines
- **CLAUDE.md** — Claude Code configuration
- **METRICS.md** — Ground truth metrics tracking
- **.rebarrc** — Tier 1 (Partial) configuration
- **.rebar-version** — Version compatibility
- **.gitignore** — Rebar-aware ignore patterns
- **architecture/** — Contract system setup
- **scripts/** — Quality enforcement tools

**Perfect for:** New projects, teams trying rebar for the first time, clean slate setups

### 📄 **component-templates/**
*Individual file templates for adding specific components*

**Contents:**
- **AGENTS.template.md** — Repository agent guidelines template
- **CLAUDE.template.md** — Claude Code configuration template
- **QUICKCONTEXT.template.md** — Project state template
- **README.template.md** — Project README template
- **STEWARD_REPORT.template.md** — Quality report template
- **TODO.template.md** — Task tracking template
- **.rebarrc.template** — Tier configuration template
- **METRICS.template** — Ground truth metrics template

**Perfect for:** Adding individual components, customizing existing setups, advanced users

---

## Usage Patterns

### 🚀 **New Project (Recommended)**
```bash
# From within rebar checkout
cp -r templates/project-bootstrap/* ../my-project/
cd ../my-project
# Customize README.md, QUICKCONTEXT.md for your project
# Start developing with contracts + agents
```

### 🔧 **Existing Project**
```bash
# Copy specific templates you need
cp templates/component-templates/AGENTS.template.md AGENTS.md
cp templates/component-templates/.rebarrc.template .rebarrc
# Customize for your existing workflow
```

### 📋 **Advanced Customization**
- Start with project-bootstrap template
- Modify files for your specific needs
- Add project-specific patterns and guidelines
- Create your own organization template

---

## Template Philosophy

### Working Examples, Not Empty Templates
- **project-bootstrap/** contains filled examples with realistic content
- Shows how rebar actually works in practice
- Ready to run immediately, then customize
- No "figure out what goes here" confusion

### Copy-and-Go Experience
- Complete project structure in one copy command
- All dependencies and configuration included
- Quality enforcement ready to use
- Agent coordination pre-configured

### Customization-Friendly
- Clear placeholders for project-specific content
- Modular structure — use what you need
- Extensible patterns for growing complexity
- Documentation explains what to change

---

**Remember:** The goal is "copy and GO" — these templates should work immediately and provide clear guidance for customization to your specific project needs.