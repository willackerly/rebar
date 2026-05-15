# Template: Security Surface Scan

> Audit a package or directory for security vulnerabilities, crypto misuse,
> and data exposure risks. Produces structured findings with severity ratings
> and remediation guidance.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | review |
| **Mode** | either (single package or fan-out across packages) |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~10K-25K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `TARGET` | yes | Package or directory to audit | `internal/auth/` |
| `THREAT_MODEL` | no | Path to threat model doc | `docs/architecture/THREAT_MODEL.md` |
| `CRYPTO_RULES` | no | Path to crypto standards | `CLAUDE.md` (Crypto rules section) |
| `OUTPUT` | no | Results path | `agents/results/security-auth.json` |

## Task

You are conducting a security surface scan of `TARGET`.

Examine every source file for vulnerabilities, focusing on the dimensions
below. For each finding, identify the exact location, explain the attack
vector, assess exploitability, and provide a concrete remediation.

## Audit Dimensions

### 1. Input Validation & Injection
- Are all external inputs validated before use (HTTP params, form data,
  file uploads, environment variables)?
- Is user input ever interpolated into SQL, shell commands, HTML, or
  template strings without sanitization?
- Are path traversal attacks prevented (no unsanitized path joins with
  user input)?
- Are deserialization inputs validated (JSON, YAML, protobuf)?

### 2. Authentication & Authorization
- Are auth checks present on every protected endpoint/function?
- Are auth checks performed server-side (not just client-side)?
- Are sessions invalidated on logout/timeout?
- Are credentials (passwords, tokens, API keys) ever logged or returned
  in responses?
- Are default credentials present in non-dev code?

### 3. Cryptographic Usage
- Are approved algorithms used (AES-256-GCM, not CBC/ECB)?
- Are nonces generated from `crypto/rand` (not `math/rand`)?
- Are nonces the correct length and never reused?
- Is key material zeroized after use?
- Are comparisons done with constant-time functions?
- Is key derivation using approved KDFs (Argon2, HKDF)?

### 4. Data Exposure
- Are sensitive values (keys, tokens, PII) ever written to logs?
- Are error messages leaking internal state (stack traces, SQL queries,
  file paths)?
- Are sensitive fields excluded from serialization (JSON, debug output)?
- Is cleartext data ever written to disk unintentionally?

### 5. Dependency & Configuration
- Are there known-vulnerable dependencies?
- Are TLS certificates validated (no `InsecureSkipVerify` in prod)?
- Are CORS policies restrictive enough?
- Are security headers present (CSP, HSTS, X-Frame-Options)?

### 6. Concurrency & Resource
- Are there race conditions on security-critical operations (TOCTOU)?
- Are resources bounded (no unbounded allocations from user input)?
- Are timeouts set on network operations?

### 7. Red Team Mode (Adversarial Mindset)

When invoked as part of a red team exercise (or whenever you want to go
deeper), adopt an attacker's perspective:

- **Cheapest path to damage:** What's the single lowest-effort attack that
  would have the highest impact? Start there.
- **Deployment surface:** Don't just audit the code — consider env vars,
  secrets in CI logs, debug endpoints left enabled, default credentials.
- **Blast radius:** For each finding, estimate the worst case if exploited.
  "Read one user's data" vs "read ALL users' data" vs "execute arbitrary code."
- **Chained attacks:** Can two low-severity findings combine into a
  high-severity attack? (e.g., info disclosure + SSRF = internal network access)
- **Supply chain:** Are dependencies pinned? Could a malicious update to a
  transitive dependency compromise the system?

## Context Files

Read these before starting:
- `QUICKCONTEXT.md` — project orientation
- `THREAT_MODEL` parameter value (if provided) — known threats and trust
  boundaries
- `CRYPTO_RULES` parameter value (if provided) — approved algorithms and
  patterns

## Output Format

```json
{
  "template": "security-surface-scan",
  "target": "<TARGET>",
  "status": "complete | partial",
  "summary": "One-line overall security posture assessment",
  "files_examined": 42,
  "findings": [
    {
      "location": "handler.go:128",
      "dimension": "input-validation",
      "severity": "critical | high | medium | low | info",
      "cwe": "CWE-89 (if applicable)",
      "finding": "User-supplied filename passed directly to os.Open without sanitization",
      "attack_vector": "Attacker can read arbitrary files via path traversal: ../../etc/passwd",
      "exploitability": "easy | moderate | difficult | theoretical",
      "remediation": "Use filepath.Clean() and validate the result is within the expected directory"
    }
  ],
  "positive_observations": [
    "Note security practices done well — reinforces good patterns"
  ],
  "risk_summary": {
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0,
    "info": 0
  }
}
```

## Success Criteria

- Every source file in `TARGET` was examined (not just entrypoints)
- Crypto usage checked against project's approved algorithms
- Every `critical` or `high` finding has a concrete `remediation`
- `positive_observations` populated (acknowledge good security practices)
- No false positives from test files or dev-only code (note context)

## Anti-Patterns

- Do NOT flag test-only code as production vulnerabilities (but DO note if
  test patterns leak into production)
- Do NOT suggest adding security features that aren't relevant to the
  threat model (e.g., CSRF protection on an API with no browser clients)
- Do NOT recommend "use a library" without specifying which library and why
- Do NOT flag intentional dev-mode insecurities (e.g., `OCIS_INSECURE=true`
  in dev compose files) as production issues — note them as "ensure not
  in prod" instead
