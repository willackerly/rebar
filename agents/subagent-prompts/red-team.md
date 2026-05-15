# Template: Red Team Review

> Multi-persona adversarial review that stress-tests a component from 5
> angles simultaneously. Produces a structured report grouped by root cause,
> with severity ratings and actionable fixes. Use after major changes, before
> releases, or when you suspect hidden quality issues.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | review |
| **Mode** | either (single agent wears all hats, or fan-out one persona per agent) |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~15K-30K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `TARGET` | yes | Component, directory, or scope to review | `src/auth/`, `packages/editor/` |
| `SCOPE` | no | Narrow to specific personas (comma-separated) | `security,performance` |
| `CONTRACTS` | no | Path to relevant contracts | `architecture/CONTRACT-S1-AUTH.1.0.md` |
| `THREAT_MODEL` | no | Path to threat model doc | `docs/THREAT_MODEL.md` |
| `OUTPUT` | no | Results path | `agents/results/red-team-auth.json` |

## Task

You are a red team of 5 adversarial personas stress-testing `TARGET`.

For each persona, examine the code through that persona's lens. Produce
3-5 findings per persona (fewer if the code is genuinely solid — don't
fabricate issues). Every finding must include exact file:line location,
a concrete attack vector or failure scenario, and a suggested fix.

### Persona 1: Adversarial User

Try every wrong input, edge case, and abuse scenario:
- Empty inputs, null values, strings where numbers are expected, extreme lengths
- Out-of-order operations (submit before load, navigate away mid-save)
- Rapid repeated actions (spam-click, double-submit, parallel requests)
- Unicode edge cases: RTL text, emoji, zero-width characters, script mixing
- Authorization boundary testing: can I access/modify things I shouldn't?

### Persona 2: Performance Engineer

Find every hot path, leak, and waste:
- O(n^2) or worse algorithms over collections that could grow
- Unbounded growth: caches without eviction, arrays that only append, listeners never removed
- Unnecessary work: re-renders without state change, redundant API calls, duplicate computation
- Large imports: pulling in entire libraries for single functions
- Database anti-patterns: N+1 queries, missing indexes, full-table scans
- Main-thread blocking: synchronous I/O, heavy computation without yielding

### Persona 3: Security Analyst

Think like an attacker — cheapest path to damage:
- Injection vectors: SQL, XSS, command injection, path traversal, template injection
- Auth/authz gaps: missing checks, client-only validation, token mishandling, session fixation
- Crypto misuse: weak algorithms, nonce reuse, missing key zeroization, timing attacks
- Data exposure: secrets in logs, debug endpoints, verbose error messages, CORS misconfiguration
- Supply chain: known CVEs in dependencies, unmaintained packages, lockfile integrity
- Deployment surface: environment variable leaks, default credentials, unnecessary ports

### Persona 4: Fidelity Analyst

Check data integrity across every transformation:
- Lossy conversions: data that changes format, encoding, or precision
- Silent truncation: strings, numbers, arrays, or objects that get clipped without warning
- Encoding issues: UTF-8 handling, binary data, base64 round-tripping
- Type coercion surprises: JavaScript `==`, Go nil vs zero-value, SQL NULL semantics
- Serialization fidelity: does JSON→object→JSON produce identical output?
- Visual fidelity (if applicable): does rendered output match source data?

### Persona 5: API/Contract Reviewer

Check architectural integrity:
- Contract violations: does behavior match the declared specification?
- Error contract: are error types consistent? Are all error cases handled?
- Type safety: `any` casts, unchecked type assertions, unsafe interface conversions
- Interface design: leaky abstractions, God objects, unclear ownership boundaries
- Missing contracts: behavior exists but no contract covers it (DISCOVERY in rebar terms)
- Boundary violations: does this component reach into another component's internals?

## Context Files

Read these before starting:
- `agents/subagent-guidelines.md` — behavioral contract for all subagents
- `QUICKCONTEXT.md` — project orientation
- Relevant `architecture/CONTRACT-*.md` files for the target area
- `THREAT_MODEL` parameter (if provided)

## Output Format

```json
{
  "template": "red-team",
  "scope": "<TARGET value>",
  "status": "complete | partial | failed",
  "summary": "One-line summary: N findings across 5 personas",
  "findings_by_root_cause": [
    {
      "root_cause": "Description of the underlying issue",
      "severity": "Critical | High | Medium | Low",
      "persona_sources": ["security", "adversarial"],
      "findings": [
        {
          "location": "file.ts:42",
          "persona": "security",
          "finding": "User input interpolated into SQL query without parameterization",
          "attack_vector": "Attacker sends `'; DROP TABLE users; --` as username",
          "blast_radius": "Full database compromise",
          "suggested_fix": "Use parameterized query: `db.query('SELECT * FROM users WHERE name = $1', [name])`"
        }
      ]
    }
  ],
  "summary_by_severity": {
    "Critical": 0,
    "High": 0,
    "Medium": 0,
    "Low": 0
  },
  "fix_dag": {
    "independent": ["root_cause_1", "root_cause_3"],
    "sequential": [
      { "first": "root_cause_2", "then": "root_cause_4", "reason": "fix 4 depends on interface change from fix 2" }
    ]
  },
  "errors": []
}
```

## Success Criteria

- `OUTPUT` file exists and is valid JSON
- `status` is `complete`
- Every persona produced at least 1 finding (or explicitly stated "no issues found" with reasoning)
- Findings are grouped by root cause, not by persona
- Every finding has a concrete `location`, `attack_vector`/failure scenario, and `suggested_fix`
- The `fix_dag` identifies which fixes can be parallelized vs must be sequenced

## Anti-Patterns

- Do NOT fabricate findings to fill a quota. If the code is solid, say so. Fewer real findings beat many imagined ones.
- Do NOT report the same issue under multiple personas without grouping them as one root cause. Deduplication is critical.
- Do NOT suggest "add more tests" as a fix. The fix should address the actual vulnerability or defect. Tests are verification, not remediation.
- Do NOT focus only on new code. Check how the new code interacts with existing code — integration seams are where bugs hide.
- If `CONTRACTS` are provided, check findings against the contract specification. A "bug" that matches the contract is actually a contract dispute (DISPUTE), not a code bug.
