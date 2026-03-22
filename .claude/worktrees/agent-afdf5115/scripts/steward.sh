#!/usr/bin/env bash
# steward.sh — Automated project health scanner
#
# Usage:
#   steward.sh              Full scan, produces JSON + markdown report
#   steward.sh --json       Aggregate JSON to stdout
#   steward.sh --summary    One-line summary to stdout
#   steward.sh --check <id> Single contract scan
#
# Dependencies: bash, jq, grep, find
# Output:
#   architecture/.state/<contract-id>.<version>.json  (per-contract)
#   architecture/.state/steward-report.json           (aggregate)
#   STEWARD_REPORT.md                                 (human-readable)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
STATE_DIR="$PROJECT_ROOT/architecture/.state"
ARCH_DIR="$PROJECT_ROOT/architecture"

# Ensure state directory exists
mkdir -p "$STATE_DIR"

# Timestamp
GENERATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

# ─── Helpers ────────────────────────────────────────────────────────────────

# Extract contract ID from filename: CONTRACT-{ID}.{MAJOR}.{MINOR}.md or CONTRACT-{ID}.md
parse_contract_file() {
  local filename
  filename="$(basename "$1")"

  # Strip CONTRACT- prefix and .md suffix
  local stem="${filename#CONTRACT-}"
  stem="${stem%.md}"

  # Check for version pattern: ID.MAJOR.MINOR
  if [[ "$stem" =~ ^(.+)\.([0-9]+)\.([0-9]+)$ ]]; then
    CONTRACT_ID="${BASH_REMATCH[1]}"
    CONTRACT_VERSION="${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
  elif [[ "$stem" =~ ^(.+)\.([0-9]+)$ ]]; then
    # ID.MAJOR only
    CONTRACT_ID="${BASH_REMATCH[1]}"
    CONTRACT_VERSION="${BASH_REMATCH[2]}.0"
  else
    # No version
    CONTRACT_ID="$stem"
    CONTRACT_VERSION="1.0"
  fi
}

# Check if a contract file has a required section
has_section() {
  local file="$1"
  local pattern="$2"
  grep -qi "$pattern" "$file" 2>/dev/null
}

# ─── Per-Contract Scan ──────────────────────────────────────────────────────

scan_contract() {
  local contract_file="$1"
  parse_contract_file "$contract_file"

  local id="$CONTRACT_ID"
  local version="$CONTRACT_VERSION"

  # Spec gate: check required sections
  local has_interfaces=false has_behavioral=false has_errors=false
  local has_tests=false has_implementing=false

  has_section "$contract_file" "## Interfaces"       && has_interfaces=true
  has_section "$contract_file" "## Behavioral"       && has_behavioral=true
  has_section "$contract_file" "## Error"            && has_errors=true
  has_section "$contract_file" "## Test"             && has_tests=true
  has_section "$contract_file" "## Implementing"     && has_implementing=true

  local completeness="pass"
  if ! $has_interfaces || ! $has_behavioral || ! $has_errors || ! $has_tests || ! $has_implementing; then
    completeness="fail"
  fi

  # Implementing files: grep for CONTRACT:<id> across the project
  local impl_files=()
  local test_files=()

  while IFS= read -r line; do
    local filepath
    filepath="$(echo "$line" | cut -d: -f1)"
    impl_files+=("$filepath")
  done < <(grep -rn "CONTRACT:${id}" "$PROJECT_ROOT" \
    --include='*.go' --include='*.ts' --include='*.js' --include='*.py' \
    --include='*.rs' --include='*.java' --include='*.rb' --include='*.c' \
    --include='*.cpp' --include='*.h' --include='*.cs' --include='*.swift' \
    --include='*.kt' --include='*.sh' --include='*.yaml' --include='*.yml' \
    --include='*.toml' --include='*.json' --include='*.vue' --include='*.svelte' \
    --include='*.jsx' --include='*.tsx' 2>/dev/null \
    | grep -v "^${ARCH_DIR}" \
    | grep -v "/.git/" \
    | grep -v "/node_modules/" \
    | sort -u || true)

  # Deduplicate impl_files by filepath
  local unique_impl=()
  local seen_files=""
  for f in "${impl_files[@]+"${impl_files[@]}"}"; do
    if [[ "$seen_files" != *"|$f|"* ]]; then
      unique_impl+=("$f")
      seen_files="${seen_files}|$f|"
    fi
  done
  impl_files=("${unique_impl[@]+"${unique_impl[@]}"}")

  # Test files: filter implementing files matching test patterns
  for f in "${impl_files[@]+"${impl_files[@]}"}"; do
    local base
    base="$(basename "$f")"
    if [[ "$base" == *_test.* ]] || [[ "$base" == *.test.* ]] || [[ "$base" == *.spec.* ]]; then
      test_files+=("$f")
    fi
  done

  local impl_count=${#impl_files[@]}
  local test_count=${#test_files[@]}

  # Lifecycle derivation
  local lifecycle="draft"
  if [ "$completeness" = "pass" ]; then
    if [ "$impl_count" -eq 0 ]; then
      lifecycle="active"
    elif [ "$test_count" -eq 0 ]; then
      lifecycle="testing"
    else
      lifecycle="verified"
    fi
  fi

  # Discovery status: parse TODO.md
  local discoveries='[]'
  local todo_file="$PROJECT_ROOT/TODO.md"
  if [ -f "$todo_file" ]; then
    local disc_lines
    disc_lines="$(grep -n "$id" "$todo_file" 2>/dev/null | grep -iE '\*\*(BUG|DISCOVERY|DRIFT|DISPUTE)\*\*' || true)"
    if [ -n "$disc_lines" ]; then
      discoveries="$(echo "$disc_lines" | while IFS= read -r line; do
        local dtype
        dtype="$(echo "$line" | grep -oiE '\*\*(BUG|DISCOVERY|DRIFT|DISPUTE)\*\*' | tr -d '*' | tr '[:upper:]' '[:lower:]' | head -1)"
        local desc
        desc="$(echo "$line" | sed 's/^[0-9]*://' | sed 's/^[[:space:]]*//')"
        jq -n --arg type "$dtype" --arg desc "$desc" '{"type": $type, "description": $desc}'
      done | jq -s '.')"
    fi
  fi

  # Build impl_files JSON array
  local impl_json='[]'
  if [ "${#impl_files[@]}" -gt 0 ]; then
    impl_json="$(printf '%s\n' "${impl_files[@]}" | jq -R '.' | jq -s '.')"
  fi

  # Build test_files JSON array
  local test_json='[]'
  if [ "${#test_files[@]}" -gt 0 ]; then
    test_json="$(printf '%s\n' "${test_files[@]}" | jq -R '.' | jq -s '.')"
  fi

  # Write per-contract JSON
  local state_file="$STATE_DIR/${id}.${version}.json"
  jq -n \
    --arg contract_id "$id" \
    --arg version "$version" \
    --arg lifecycle "$lifecycle" \
    --argjson has_interfaces "$has_interfaces" \
    --argjson has_behavioral "$has_behavioral" \
    --argjson has_errors "$has_errors" \
    --argjson has_tests "$has_tests" \
    --argjson has_implementing "$has_implementing" \
    --arg completeness "$completeness" \
    --argjson impl_files "$impl_json" \
    --argjson test_files "$test_json" \
    --argjson impl_count "$impl_count" \
    --argjson test_count "$test_count" \
    --argjson discoveries "$discoveries" \
    '{
      contract_id: $contract_id,
      version: $version,
      lifecycle: $lifecycle,
      spec_gate: {
        has_interfaces: $has_interfaces,
        has_behavioral: $has_behavioral,
        has_errors: $has_errors,
        has_tests: $has_tests,
        has_implementing: $has_implementing,
        completeness: $completeness
      },
      impl_gate: {
        implementing_files: $impl_files,
        test_files: $test_files,
        implementing_count: $impl_count,
        test_count: $test_count
      },
      discoveries: $discoveries
    }' > "$state_file"

  # Output the path for the caller
  echo "$state_file"
}

# ─── Enforcement Checks ────────────────────────────────────────────────────

run_enforcement() {
  local enforcement='{}'

  local checks=(
    "contract_headers:check-contract-headers.sh"
    "contract_refs:check-contract-refs.sh"
    "todo_tracking:check-todos.sh"
    "doc_freshness:check-freshness.sh"
    "registry:check-registry.sh"
    "ground_truth:check-ground-truth.sh"
  )

  local passing=0
  local total=0

  for entry in "${checks[@]}"; do
    local key="${entry%%:*}"
    local script="${entry##*:}"
    local script_path="$SCRIPT_DIR/$script"
    total=$((total + 1))

    local result="skip"
    if [ -x "$script_path" ]; then
      if "$script_path" >/dev/null 2>&1; then
        result="pass"
        passing=$((passing + 1))
      else
        result="fail"
      fi
    fi

    enforcement="$(echo "$enforcement" | jq --arg k "$key" --arg v "$result" '. + {($k): $v}')"
  done

  ENFORCEMENT_JSON="$enforcement"
  ENFORCEMENT_PASSING="$passing"
  ENFORCEMENT_TOTAL="$total"
}

# ─── Action Item Generation ────────────────────────────────────────────────

generate_action_items() {
  local contracts_json="$1"

  local architect='[]'
  local englead='[]'
  local product='[]'
  local dev='[]'

  # Process each contract
  local count
  count="$(echo "$contracts_json" | jq 'length')"

  for ((i = 0; i < count; i++)); do
    local c
    c="$(echo "$contracts_json" | jq ".[$i]")"
    local cid lifecycle completeness
    cid="$(echo "$c" | jq -r '.contract_id')"
    lifecycle="$(echo "$c" | jq -r '.lifecycle')"
    completeness="$(echo "$c" | jq -r '.spec_gate.completeness')"

    # Missing sections for draft contracts
    if [ "$lifecycle" = "draft" ]; then
      local missing=()
      [ "$(echo "$c" | jq '.spec_gate.has_interfaces')" = "false" ] && missing+=("Interfaces")
      [ "$(echo "$c" | jq '.spec_gate.has_behavioral')" = "false" ] && missing+=("Behavioral")
      [ "$(echo "$c" | jq '.spec_gate.has_errors')" = "false" ] && missing+=("Errors")
      [ "$(echo "$c" | jq '.spec_gate.has_tests')" = "false" ] && missing+=("Tests")
      [ "$(echo "$c" | jq '.spec_gate.has_implementing')" = "false" ] && missing+=("Implementing Files")
      local missing_str
      missing_str="$(IFS=', '; echo "${missing[*]}")"
      architect="$(echo "$architect" | jq --arg msg "$cid is DRAFT — missing sections: $missing_str" '. + [$msg]')"
    fi

    # Disputes → architect
    local dispute_count
    dispute_count="$(echo "$c" | jq '[.discoveries[] | select(.type == "dispute")] | length')"
    if [ "$dispute_count" -gt 0 ]; then
      architect="$(echo "$architect" | jq --arg msg "$cid has $dispute_count DISPUTE discovery(ies) — needs resolution" '. + [$msg]')"
    fi

    # TESTING → englead
    if [ "$lifecycle" = "testing" ]; then
      englead="$(echo "$englead" | jq --arg msg "$cid is TESTING — needs test files for verification" '. + [$msg]')"
    fi

    # DISCOVERY → product
    local discovery_count
    discovery_count="$(echo "$c" | jq '[.discoveries[] | select(.type == "discovery")] | length')"
    if [ "$discovery_count" -gt 0 ]; then
      product="$(echo "$product" | jq --arg msg "$cid has $discovery_count DISCOVERY item(s) — needs triage" '. + [$msg]')"
    fi

    # ACTIVE → dev (needs implementation)
    if [ "$lifecycle" = "active" ]; then
      dev="$(echo "$dev" | jq --arg msg "$cid is ACTIVE — needs implementing files" '. + [$msg]')"
    fi

    # TESTING → dev (needs more tests)
    if [ "$lifecycle" = "testing" ]; then
      dev="$(echo "$dev" | jq --arg msg "$cid is TESTING — needs test files" '. + [$msg]')"
    fi
  done

  # Enforcement failures → englead
  if [ "$ENFORCEMENT_PASSING" -lt "$ENFORCEMENT_TOTAL" ]; then
    local fail_count=$((ENFORCEMENT_TOTAL - ENFORCEMENT_PASSING))
    englead="$(echo "$englead" | jq --arg msg "$fail_count enforcement check(s) failing" '. + [$msg]')"
  fi

  ACTION_ARCHITECT="$architect"
  ACTION_ENGLEAD="$englead"
  ACTION_PRODUCT="$product"
  ACTION_DEV="$dev"
}

# ─── Aggregate Report ──────────────────────────────────────────────────────

build_aggregate() {
  local contracts_json="$1"

  local total draft active testing verified open_discoveries
  total="$(echo "$contracts_json" | jq 'length')"
  draft="$(echo "$contracts_json" | jq '[.[] | select(.lifecycle == "draft")] | length')"
  active="$(echo "$contracts_json" | jq '[.[] | select(.lifecycle == "active")] | length')"
  testing="$(echo "$contracts_json" | jq '[.[] | select(.lifecycle == "testing")] | length')"
  verified="$(echo "$contracts_json" | jq '[.[] | select(.lifecycle == "verified")] | length')"
  open_discoveries="$(echo "$contracts_json" | jq '[.[].discoveries[]] | length')"

  jq -n \
    --arg generated_at "$GENERATED_AT" \
    --argjson total "$total" \
    --argjson draft "$draft" \
    --argjson active "$active" \
    --argjson testing "$testing" \
    --argjson verified "$verified" \
    --argjson open_discoveries "$open_discoveries" \
    --argjson passing "$ENFORCEMENT_PASSING" \
    --argjson enf_total "$ENFORCEMENT_TOTAL" \
    --argjson contracts "$contracts_json" \
    --argjson architect "$ACTION_ARCHITECT" \
    --argjson englead "$ACTION_ENGLEAD" \
    --argjson product "$ACTION_PRODUCT" \
    --argjson dev "$ACTION_DEV" \
    --argjson enforcement "$ENFORCEMENT_JSON" \
    '{
      generated_at: $generated_at,
      summary: {
        contracts: {
          total: $total,
          draft: $draft,
          active: $active,
          testing: $testing,
          verified: $verified
        },
        open_discoveries: $open_discoveries,
        enforcement: {
          passing: $passing,
          total: $enf_total
        }
      },
      contracts: $contracts,
      action_items: {
        architect: $architect,
        englead: $englead,
        product: $product,
        dev: $dev
      },
      enforcement: $enforcement
    }'
}

# ─── Markdown Report ───────────────────────────────────────────────────────

generate_markdown() {
  local report_json="$1"
  local output="$PROJECT_ROOT/STEWARD_REPORT.md"

  local total draft active testing verified open_disc enf_pass enf_total
  total="$(echo "$report_json" | jq '.summary.contracts.total')"
  draft="$(echo "$report_json" | jq '.summary.contracts.draft')"
  active="$(echo "$report_json" | jq '.summary.contracts.active')"
  testing="$(echo "$report_json" | jq '.summary.contracts.testing')"
  verified="$(echo "$report_json" | jq '.summary.contracts.verified')"
  open_disc="$(echo "$report_json" | jq '.summary.open_discoveries')"
  enf_pass="$(echo "$report_json" | jq '.summary.enforcement.passing')"
  enf_total="$(echo "$report_json" | jq '.summary.enforcement.total')"

  {
    echo "# Steward Report"
    echo ""
    echo "<!-- Generated by scripts/steward.sh — do not edit manually -->"
    echo "<!-- freshness: $GENERATED_AT -->"
    echo ""
    echo "## Summary"
    echo ""
    echo "| Metric | Value |"
    echo "|--------|-------|"
    echo "| Contracts | $total total ($draft draft, $active active, $testing testing, $verified verified) |"
    echo "| Open Discoveries | $open_disc |"
    echo "| Enforcement | $enf_pass/$enf_total passing |"
    echo ""

    # Contract Status
    echo "## Contract Status"
    echo ""
    echo "| Contract | Version | Lifecycle | Spec Gate | Impl Files | Test Files | Discoveries |"
    echo "|----------|---------|-----------|-----------|------------|------------|-------------|"

    if [ "$total" -eq 0 ]; then
      echo "| _(none)_ | | | | | | |"
    else
      echo "$report_json" | jq -r '.contracts[] | "| \(.contract_id) | \(.version) | \(.lifecycle) | \(.spec_gate.completeness) | \(.impl_gate.implementing_count) | \(.impl_gate.test_count) | \(.discoveries | length) |"'
    fi

    echo ""

    # Action Items
    echo "## Action Items"
    echo ""

    echo "### Architect"
    local arch_count
    arch_count="$(echo "$report_json" | jq '.action_items.architect | length')"
    if [ "$arch_count" -eq 0 ]; then
      echo "- _(no items)_"
    else
      echo "$report_json" | jq -r '.action_items.architect[] | "- \(.)"'
    fi
    echo ""

    echo "### Engineering Lead"
    local eng_count
    eng_count="$(echo "$report_json" | jq '.action_items.englead | length')"
    if [ "$eng_count" -eq 0 ]; then
      echo "- _(no items)_"
    else
      echo "$report_json" | jq -r '.action_items.englead[] | "- \(.)"'
    fi
    echo ""

    echo "### Product"
    local prod_count
    prod_count="$(echo "$report_json" | jq '.action_items.product | length')"
    if [ "$prod_count" -eq 0 ]; then
      echo "- _(no items)_"
    else
      echo "$report_json" | jq -r '.action_items.product[] | "- \(.)"'
    fi
    echo ""

    echo "### Developer"
    local dev_count
    dev_count="$(echo "$report_json" | jq '.action_items.dev | length')"
    if [ "$dev_count" -eq 0 ]; then
      echo "- _(no items)_"
    else
      echo "$report_json" | jq -r '.action_items.dev[] | "- \(.)"'
    fi
    echo ""

    # Open Discoveries
    echo "## Open Discoveries"
    echo ""
    echo "_(See TODO.md Discoveries section for details)_"
    echo ""
    echo "| Type | Contract | Description |"
    echo "|------|----------|-------------|"

    local disc_count
    disc_count="$(echo "$report_json" | jq '[.contracts[].discoveries[]] | length')"
    if [ "$disc_count" -eq 0 ]; then
      echo "| _(none)_ | | |"
    else
      echo "$report_json" | jq -r '.contracts[] as $c | $c.discoveries[] | "| \(.type) | \($c.contract_id) | \(.description) |"'
    fi

    echo ""

    # Enforcement Results
    echo "## Enforcement Results"
    echo ""
    echo "| Check | Result |"
    echo "|-------|--------|"

    local check_names=(
      "contract_headers:Contract Headers"
      "contract_refs:Contract References"
      "todo_tracking:TODO Tracking"
      "doc_freshness:Doc Freshness"
      "registry:Registry Consistency"
      "ground_truth:Ground Truth"
    )

    for entry in "${check_names[@]}"; do
      local key="${entry%%:*}"
      local label="${entry##*:}"
      local result
      result="$(echo "$report_json" | jq -r ".enforcement.${key} // \"_(not run)_\"")"
      echo "| $label | $result |"
    done

    echo ""
    echo "---"
    echo ""
    echo "_Generated at ${GENERATED_AT} by \`scripts/steward.sh\`_"
  } > "$output"
}

# ─── Main ───────────────────────────────────────────────────────────────────

# Globals set by run_enforcement / generate_action_items
ENFORCEMENT_JSON='{}'
ENFORCEMENT_PASSING=0
ENFORCEMENT_TOTAL=0
ACTION_ARCHITECT='[]'
ACTION_ENGLEAD='[]'
ACTION_PRODUCT='[]'
ACTION_DEV='[]'

main() {
  local mode="${1:-full}"
  local check_id="${2:-}"

  # Single contract check
  if [ "$mode" = "--check" ]; then
    if [ -z "$check_id" ]; then
      echo "Usage: steward.sh --check <contract-id>" >&2
      exit 1
    fi
    # Find contract file
    local contract_file
    contract_file="$(ls "$ARCH_DIR"/CONTRACT-"${check_id}"*.md 2>/dev/null | head -1 || true)"
    if [ -z "$contract_file" ]; then
      echo "Contract not found: $check_id" >&2
      exit 1
    fi
    scan_contract "$contract_file"
    cat "$STATE_DIR/${check_id}."*.json 2>/dev/null | jq .
    exit 0
  fi

  # Full scan: collect all contracts
  local all_contracts='[]'
  local contract_files=()

  while IFS= read -r f; do
    contract_files+=("$f")
  done < <(ls "$ARCH_DIR"/CONTRACT-*.md 2>/dev/null | grep -v TEMPLATE | grep -v REGISTRY || true)

  for contract_file in "${contract_files[@]+"${contract_files[@]}"}"; do
    local state_file
    state_file="$(scan_contract "$contract_file")"
    local contract_json
    contract_json="$(cat "$state_file")"
    all_contracts="$(echo "$all_contracts" | jq --argjson c "$contract_json" '. + [$c]')"
  done

  # Run enforcement checks
  run_enforcement

  # Generate action items
  generate_action_items "$all_contracts"

  # Build aggregate report
  local report
  report="$(build_aggregate "$all_contracts")"

  # Write aggregate JSON
  echo "$report" | jq . > "$STATE_DIR/steward-report.json"

  case "$mode" in
    --json)
      echo "$report" | jq .
      ;;
    --summary)
      local total draft active testing verified open_disc enf_pass enf_total
      total="$(echo "$report" | jq '.summary.contracts.total')"
      draft="$(echo "$report" | jq '.summary.contracts.draft')"
      active="$(echo "$report" | jq '.summary.contracts.active')"
      testing="$(echo "$report" | jq '.summary.contracts.testing')"
      verified="$(echo "$report" | jq '.summary.contracts.verified')"
      open_disc="$(echo "$report" | jq '.summary.open_discoveries')"
      enf_pass="$(echo "$report" | jq '.summary.enforcement.passing')"
      enf_total="$(echo "$report" | jq '.summary.enforcement.total')"
      echo "Steward: ${total} contracts (${draft}d/${active}a/${testing}t/${verified}v), ${open_disc} discoveries, ${enf_pass}/${enf_total} enforcement passing"
      ;;
    full|*)
      # Generate markdown report
      generate_markdown "$report"
      echo "Steward scan complete."
      echo "  JSON:     $STATE_DIR/steward-report.json"
      echo "  Markdown: $PROJECT_ROOT/STEWARD_REPORT.md"
      echo ""
      # Print summary line
      local total
      total="$(echo "$report" | jq '.summary.contracts.total')"
      local enf_pass enf_total
      enf_pass="$(echo "$report" | jq '.summary.enforcement.passing')"
      enf_total="$(echo "$report" | jq '.summary.enforcement.total')"
      local open_disc
      open_disc="$(echo "$report" | jq '.summary.open_discoveries')"
      echo "  ${total} contracts, ${open_disc} discoveries, ${enf_pass}/${enf_total} enforcement passing"
      ;;
  esac
}

main "$@"
