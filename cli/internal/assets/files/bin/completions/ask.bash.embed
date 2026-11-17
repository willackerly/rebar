# bash completion for the `ask` CLI
#
# Install:
#   source bin/completions/ask.bash
# Or system-wide (via bin/install):
#   ln -s "$(pwd)/bin/completions/ask.bash" /usr/local/etc/bash_completion.d/ask
#
# Completes:
# - subcommands (who, log, peek, init, register, ...)
# - agent names from ./agents/*/AGENT.md (case-insensitive)
# - cross-repo project names from ~/.config/ask/projects

_ask_completions() {
  local cur prev words cword
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  local subcommands="help who where status log peek tail watch up reset compact register projects init serve agents"
  local flags="-v -d -w -V --verbose --debug --write --version --help -h"

  # First positional: a subcommand, an agent, or a flag
  if [ "$COMP_CWORD" -eq 1 ]; then
    local agents=""
    if [ -d "./agents" ]; then
      agents=$(find ./agents -maxdepth 2 -name AGENT.md -exec dirname {} \; 2>/dev/null \
        | xargs -n1 basename 2>/dev/null | tr '\n' ' ')
    fi
    COMPREPLY=( $(compgen -W "$subcommands $agents $flags" -- "$cur") )
    return 0
  fi

  # After a subcommand that takes an agent name
  case "$prev" in
    where|status|log|peek|watch|up|reset|compact|tail)
      if [ -d "./agents" ]; then
        local agents
        agents=$(find ./agents -maxdepth 2 -name AGENT.md -exec dirname {} \; 2>/dev/null \
          | xargs -n1 basename 2>/dev/null | tr '\n' ' ')
        COMPREPLY=( $(compgen -W "$agents" -- "$cur") )
      fi
      return 0
      ;;
  esac

  # Default: don't complete (the next arg is a free-form question)
  return 0
}

complete -F _ask_completions ask
