#!/usr/bin/env bash
set -euo pipefail

# Rebar Setup Script
# Automates the installation of github.com/willackerly/rebar for contract-driven development
# Based on the quickstart guide: https://github.com/willackerly/rebar/blob/main/QUICKSTART.md

# Configuration
REBAR_REPO="https://github.com/willackerly/rebar.git"
REBAR_DIR="rebar"
ASK_SERVER=""
INSTALL_DIR=""
VERBOSE=false

# Colors for output
RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m' # No Color

# Logging functions
log() { echo -e "${BLUE}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }

# Show usage information
usage() {
    cat << EOF
Rebar Setup Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -d, --dir PATH          Directory to install rebar and set up project in (default: current directory)
    -s, --server HOST:PORT  ASK server for remote agent access (optional)
    -v, --verbose          Enable verbose output
    -h, --help             Show this help message

EXAMPLES:
    # Basic setup - sets up rebar in current directory
    $0
    
    # Setup with custom directory  
    $0 --dir /path/to/workspace
    
    # Setup with ASK server for remote agents
    $0 --server 192.168.0.181:7232
    
    # Verbose output
    $0 --verbose

WHAT THIS SCRIPT DOES:
    1. Checks and installs missing dependencies (auto-installs via Homebrew on macOS)
    2. Clones the rebar repository
    3. Copies project bootstrap template to current directory
    4. Installs rebar CLI tools and adds to PATH
    5. Sets up ASK server configuration (if specified)
    6. Verifies installation and runs initial checks

DEPENDENCIES:
    - git, bash 4.0+, jq, python3 (auto-installed on macOS)
    - claude CLI (optional, for full agent functionality)
    - On macOS: Homebrew will be installed if not present

For more information, see: https://github.com/willackerly/rebar
EOF
}

# Check if we're on macOS
is_macos() {
    [[ "$(uname)" == "Darwin" ]]
}

# Install homebrew if not present (macOS only)
install_homebrew() {
    if ! command -v brew &> /dev/null; then
        log "Homebrew not found. Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        
        # Add brew to PATH for current session
        if [[ -d "/opt/homebrew" ]]; then
            export PATH="/opt/homebrew/bin:$PATH"
        elif [[ -d "/usr/local/Homebrew" ]]; then
            export PATH="/usr/local/bin:$PATH" 
        fi
        
        if ! command -v brew &> /dev/null; then
            error "Failed to install or locate Homebrew. Please install manually and try again."
            exit 1
        fi
        success "Homebrew installed successfully"
    fi
}

# Install missing dependency using appropriate package manager
install_dependency() {
    local dep="$1"
    
    if is_macos; then
        install_homebrew
        
        case "$dep" in
            "bash")
                log "Installing bash 5.x to meet version requirement..."
                brew install bash
                # Add new bash to /etc/shells if not already there
                local new_bash_path="$(brew --prefix)/bin/bash"
                if [[ -f "$new_bash_path" ]] && ! grep -q "$new_bash_path" /etc/shells; then
                    echo "$new_bash_path" | sudo tee -a /etc/shells
                fi
                warn "New bash installed at $new_bash_path"
                warn "You may want to change your default shell: chsh -s $new_bash_path"
                ;;
            "git")
                log "Installing git..."
                brew install git
                ;;
            "jq")
                log "Installing jq..."
                brew install jq
                ;;
            "python3")
                log "Installing python3..."
                brew install python3
                ;;
            *)
                warn "Unknown dependency: $dep. Please install manually."
                return 1
                ;;
        esac
    else
        warn "Automatic dependency installation is only supported on macOS."
        warn "Please install $dep manually and try again."
        return 1
    fi
}

# Check if required commands exist
check_dependencies() {
    log "Checking dependencies..."
    
    local missing_deps=()
    local need_bash_upgrade=false
    
    # Check for missing commands
    for cmd in git jq python3; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_deps+=("$cmd")
        fi
    done
    
    # Special handling for bash version
    if ! command -v bash &> /dev/null; then
        missing_deps+=("bash")
    elif [[ ${BASH_VERSION%%.*} -lt 4 ]]; then
        need_bash_upgrade=true
        missing_deps+=("bash")
        warn "Current bash version ($BASH_VERSION) is too old. Need 4.0+."
    fi
    
    # Check Claude CLI
    if ! command -v claude &> /dev/null; then
        warn "Claude CLI not found. You may need to install it for full agent functionality."
        warn "See: https://docs.anthropic.com/claude/docs/cli for installation instructions"
    fi
    
    # Attempt to install missing dependencies on macOS
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        if is_macos; then
            log "Missing dependencies detected: ${missing_deps[*]}"
            log "Attempting to install using Homebrew..."
            
            local install_failed=false
            for dep in "${missing_deps[@]}"; do
                if ! install_dependency "$dep"; then
                    install_failed=true
                fi
            done
            
            if [[ "$install_failed" == true ]]; then
                error "Some dependencies could not be installed automatically."
                error "Please install them manually and try again."
                exit 1
            fi
            
            # Re-check bash version if it was upgraded
            if [[ "$need_bash_upgrade" == true ]]; then
                # Use the new bash to check version
                local new_bash_path
                if command -v "$(brew --prefix)/bin/bash" &> /dev/null; then
                    new_bash_path="$(brew --prefix)/bin/bash"
                    local new_version=$("$new_bash_path" --version | head -1 | sed 's/.*version \([0-9]*\).*/\1/')
                    if [[ $new_version -ge 4 ]]; then
                        success "Bash upgraded to version $new_version"
                        warn "Note: You're still using the old bash in this session."
                        warn "Restart your terminal or run: export PATH=\"$(brew --prefix)/bin:\$PATH\""
                    else
                        error "Bash upgrade failed. Please check your installation."
                        exit 1
                    fi
                fi
            fi
        else
            error "Missing required dependencies: ${missing_deps[*]}"
            error "Please install the missing commands and try again."
            if [[ "$need_bash_upgrade" == true ]]; then
                error "Your bash version ($BASH_VERSION) is too old. Need 4.0+."
            fi
            exit 1
        fi
    fi
    
    # Final version check for current bash session
    if [[ ${BASH_VERSION%%.*} -lt 4 ]] && [[ "$need_bash_upgrade" == true ]]; then
        warn "Current session is still using old bash ($BASH_VERSION)."
        warn "The script will continue, but you may want to restart your terminal."
    fi
    
    success "All dependencies satisfied"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -d|--dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            -s|--server)
                ASK_SERVER="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    # Set default install directory if not specified
    if [[ -z "$INSTALL_DIR" ]]; then
        INSTALL_DIR="$(pwd)"
    fi
    
    # Make install directory absolute
    INSTALL_DIR="$(cd "$INSTALL_DIR" && pwd)"
}

# Clone rebar repository
clone_rebar() {
    log "Cloning rebar repository..."
    
    cd "$INSTALL_DIR"
    
    if [[ -d "$REBAR_DIR" ]]; then
        warn "Rebar directory already exists. Updating..."
        cd "$REBAR_DIR"
        git pull origin main
        cd "$INSTALL_DIR"
    else
        git clone "$REBAR_REPO" "$REBAR_DIR"
    fi
    
    success "Rebar repository ready at $INSTALL_DIR/$REBAR_DIR"
}

# Copy project bootstrap template
setup_project() {
    log "Setting up current directory with rebar bootstrap..."
    
    local project_dir="$INSTALL_DIR"
    local project_name=$(basename "$project_dir")
    
    local template_dir="$INSTALL_DIR/$REBAR_DIR/templates/project-bootstrap"
    local skipped=()

    # Copy each template file only if it doesn't already exist in the target.
    while IFS= read -r -d '' src; do
        local rel="${src#$template_dir/}"
        local dst="$project_dir/$rel"

        if [[ -e "$dst" ]]; then
            skipped+=("$rel")
        else
            mkdir -p "$(dirname "$dst")"
            cp "$src" "$dst"
            [[ "$VERBOSE" == true ]] && log "  copied: $rel"
        fi
    done < <(find "$template_dir" -type f -print0)

    if [[ ${#skipped[@]} -gt 0 ]]; then
        warn "The following files already exist and were NOT overwritten:"
        for f in "${skipped[@]}"; do
            warn "  $f"
        done
    fi

    # Update project name in README only if it was just created (not skipped).
    local readme="$project_dir/README.md"
    if [[ -f "$readme" ]] && ! printf '%s\n' "${skipped[@]}" | grep -qx "README.md"; then
        sed -i.bak "s/My Project/$project_name/g" "$readme"
        rm "$readme.bak"
    fi

    success "Rebar bootstrap applied to current directory: $project_dir"
}

# Install rebar CLI tools
install_cli_tools() {
    log "Installing rebar CLI tools..."
    
    cd "$INSTALL_DIR/$REBAR_DIR"
    
    # Build install command
    local install_cmd="./bin/install"
    if [[ -n "$ASK_SERVER" ]]; then
        install_cmd="$install_cmd --server $ASK_SERVER"
    fi
    
    # Run installer
    if [[ "$VERBOSE" == true ]]; then
        log "Running: $install_cmd"
    fi
    
    $install_cmd
    
    success "Rebar CLI tools installed"
}

# Install pre-commit hook from rebar scripts
install_pre_commit_hook() {
    local project_dir="$INSTALL_DIR"
    local git_dir="$project_dir/.git"
    local hook_src="$project_dir/scripts/pre-commit.sh"
    local hook_dst="$git_dir/hooks/pre-commit"

    if [[ ! -d "$git_dir" ]]; then
        warn "No .git directory found at $project_dir — skipping pre-commit hook installation"
        return
    fi

    if [[ ! -f "$hook_src" ]]; then
        warn "scripts/pre-commit.sh not found — skipping pre-commit hook installation"
        return
    fi

    if [[ -L "$hook_dst" && "$(readlink "$hook_dst")" == "../../scripts/pre-commit.sh" ]]; then
        log "Pre-commit hook already installed, skipping"
        return
    fi

    mkdir -p "$git_dir/hooks"
    ln -sf "../../scripts/pre-commit.sh" "$hook_dst"
    chmod +x "$hook_src"
    success "Pre-commit hook installed: .git/hooks/pre-commit → scripts/pre-commit.sh"
}

# Verify installation
verify_installation() {
    log "Verifying installation..."
    
    # Source shell profile to get updated PATH
    if [[ -f "$HOME/.zshrc" ]]; then
        source "$HOME/.zshrc" 2>/dev/null || true
    elif [[ -f "$HOME/.bashrc" ]]; then
        source "$HOME/.bashrc" 2>/dev/null || true
    elif [[ -f "$HOME/.profile" ]]; then
        source "$HOME/.profile" 2>/dev/null || true
    fi
    
    local project_dir="$INSTALL_DIR"
    cd "$project_dir"
    
    # Check if scripts are executable
    if [[ -x "scripts/check-contract-refs.sh" ]]; then
        log "Running contract reference check..."
        ./scripts/check-contract-refs.sh
    fi
    
    # Test ASK command if available
    if command -v ask &> /dev/null; then
        log "Testing ASK CLI..."
        ask who || warn "ASK CLI installed but agents not yet initialized"
    else
        warn "ASK command not found in PATH. You may need to restart your shell or source your profile."
    fi
    
    success "Installation verification complete"
}

# Show completion message and next steps
show_completion() {
    local project_dir="$INSTALL_DIR"
    
    cat << EOF

${GREEN}🎉 Rebar setup complete!${NC}

${BLUE}Project directory:${NC} $project_dir
${BLUE}Rebar location:${NC} $INSTALL_DIR/$REBAR_DIR

${YELLOW}Next steps:${NC}
1. Navigate to your project: ${BLUE}cd $project_dir${NC}
2. Restart your shell or run: ${BLUE}source ~/.zshrc${NC} (or ~/.bashrc)
3. Verify contract links: ${BLUE}./scripts/check-contract-refs.sh${NC}
4. Test ASK CLI: ${BLUE}ask who${NC}
5. Create your first contract and ask an agent for review

${YELLOW}Key files to explore:${NC}
- ${BLUE}README.md${NC} - Project overview and cold start guide
- ${BLUE}QUICKCONTEXT.md${NC} - Current project state
- ${BLUE}TODO.md${NC} - Active work and known issues
- ${BLUE}AGENTS.md${NC} - How to work with AI agents
- ${BLUE}architecture/${NC} - Contract specifications

${YELLOW}Learn more:${NC}
- Feature development: ${BLUE}https://github.com/willackerly/rebar/blob/main/FEATURE-DEVELOPMENT.md${NC}
- Agent coordination: ${BLUE}https://github.com/willackerly/rebar/blob/main/AGENTS-QUICKSTART.md${NC}
- Case studies: ${BLUE}https://github.com/willackerly/rebar/blob/main/CASE-STUDIES.md${NC}

${GREEN}Happy coding with rebar! 🔨${NC}
EOF
}

# Main execution
main() {
    log "Starting rebar setup..."
    
    parse_args "$@"
    
    if [[ "$VERBOSE" == true ]]; then
        log "Configuration:"
        log "  Project directory: $INSTALL_DIR"
        log "  ASK server: ${ASK_SERVER:-"(local only)"}"
    fi
    
    check_dependencies
    clone_rebar
    setup_project
    install_pre_commit_hook
    install_cli_tools
    verify_installation
    show_completion
    
    success "Rebar setup completed successfully!"
}

# Run main function with all arguments
main "$@"
