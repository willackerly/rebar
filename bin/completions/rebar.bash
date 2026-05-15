# bash completion for the `rebar` CLI
#
# Install:
#   source bin/completions/rebar.bash
# Or system-wide (via bin/install):
#   ln -s "$(pwd)/bin/completions/rebar.bash" /usr/local/etc/bash_completion.d/rebar

_rebar_completions() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  # Subcommands grouped by purpose (matches the cobra cmd groups in
  # cli/cmd/root.go). Keep in sync if new commands land.
  local commands="new adopt init context commit audit push ask agent verify status check diff contract sign key version help"
  local flags="--verbose -v --json --repo-root --version --help -h"

  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "$commands $flags" -- "$cur") )
    return 0
  fi

  # Context role completion: rebar context <role>
  if [ "${COMP_WORDS[1]}" = "context" ] && [ "$COMP_CWORD" -eq 2 ]; then
    COMPREPLY=( $(compgen -W "architect product security developer session-start" -- "$cur") )
    return 0
  fi

  # rebar ask <agent> — defer to the ask completion if loaded
  if [ "${COMP_WORDS[1]}" = "ask" ] && [ "$COMP_CWORD" -eq 2 ]; then
    if [ -d "./agents" ]; then
      local agents
      agents=$(find ./agents -maxdepth 2 -name AGENT.md -exec dirname {} \; 2>/dev/null \
        | xargs -n1 basename 2>/dev/null | tr '\n' ' ')
      COMPREPLY=( $(compgen -W "$agents" -- "$cur") )
    fi
    return 0
  fi

  return 0
}

complete -F _rebar_completions rebar
