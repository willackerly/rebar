# TDFLite Entitlement Consumption — Ask from filedag

> **From:** filedag (Will, Federation Architect)
> **To:** TDFLite Architect
> **Date:** 2026-04-19
> **Type:** Ask + forward-looking integration feedback
> **Context:** filedag is implementing `internal/abac/` with a pluggable `EntitlementSource` interface. Want to plug TDFLite in as the first real (non-config-file) implementation. Reading the TDFLite README, ARCHITECTURE.md, and CLIENT_CONCEPTS.md gave us the shape; this ask surfaces the integration questions we need answered before wiring it in.

## What we understood (confirm or correct)

From `docs/ARCHITECTURE.md` + `docs/CLIENT_CONCEPTS.md`:

1. **Policy bundle** (`policy.sealed.json`) holds the ground-truth mapping of **identities → attribute claims**. Sealed with an SSH key or passphrase. On `tdflite serve`, identities + attribute definitions provision into embedded Postgres via platform migrations.
2. **Runtime entitlement delivery** happens via **OIDC JWT claims**, not a REST API lookup:
   - Client authenticates (password grant for interactive; client_credentials for services/NPEs)
   - idplite issues a JWT with `preferred_username`, `client_id`, `realm_access.roles`, plus **arbitrary custom claims** (e.g., `classification_level`, `department`, `compliance`)
   - Services/resource-servers validate the JWT via JWKS, read claims directly
   - Platform `services.entityresolution.mode: claims` bypasses any Keycloak-style user-lookup; the JWT IS the entitlement document at request time
3. **Attribute FQN format:** `https://<namespace>/attr/<name>/value/<value>` (e.g., `https://tdflite.local/attr/classification/value/top-secret`)
4. **Subject mapping selectors:** top-level JWT claim (`.classification_level`), not nested (`.attributes.classification_level[]` like Keycloak)

If we have that wrong, please correct before we wire.

## filedag's consumption shape (what we plan to build)

```go
// internal/abac/entitlements.go
type Membership struct {
    ID       string    // e.g., "admin", "engineering-department"
    Scope    []string  // FQN-flavored: ["classification:top-secret", "department:engineering"]
    Tier     *string   // derived from attributes (e.g., "medical-provider")
    Expiry   *int64    // token exp claim
    Issuer   string    // token iss claim
}

type EntitlementSource interface {
    LookupMemberships(ctx context.Context, identity string) ([]Membership, error)
}

// Implementations:
type ConfigEntitlementSource struct{ policyPath string }  // Phase 1 local-dev: reads policy.sealed.json claims directly

type TDFLiteOIDCEntitlementSource struct {
    IssuerURL string                   // e.g., http://localhost:15433
    Audience  string
    JWKSCache jwks.Cache
}  // Phase 2: standard OIDC JWT validator + claims-to-memberships translator
```

**The interesting part:** we don't want a `LookupMemberships(identity)` REST round-trip per request. The JWT carries everything; we want to extract memberships from the presented token, not re-fetch. So our `TDFLiteOIDC` impl is really a **JWT claim extractor**: take the bearer token on the incoming request, validate signature against JWKS, extract custom claims, map to `[]Membership`.

## Ask (specific questions)

1. **Canonical custom-claim schema for a filedag-style workload.** TDFLite's README examples use `classification` (hierarchy) + `department` (anyOf) + `compliance` (allOf). filedag's attributes are different (`rating` G/PG/R/X, `domain` medical/work, `release_scope`, `phi`, `sensitivity`). Should we:

   - Define each as a TDFLite attribute in `policy.json` with an appropriate rule? (Our guess: `rating` = hierarchy, `release_scope` = anyOf, `phi` = boolean-as-allOf, `domain` = anyOf.)
   - Or is there a recommended taxonomy for this kind of content-classification domain that avoids re-inventing?

2. **Recommended Go OIDC client.** Is `github.com/coreos/go-oidc/v3` the idiomatic choice in the OpenTDF / TDFLite ecosystem, or does the platform provide a lighter-weight client we should consume? (We saw `lestrrat-go/jwx/v2` used inside TDFLite itself.)

3. **Claims → memberships translation pattern.** Is there an existing library or reference implementation for "flatten JWT custom claims into a list of scopes/memberships"? Or do consumers just grab the map and map it themselves? An example output contract would unblock us.

4. **NPE (non-person entity) auth via certificates.** Will wants explicit NPE cert auth for filedag's pipeline daemon, scanner, and crawlers (service-to-service, no human in the loop). TDFLite supports `client_credentials`; does OpenTDF/TDFLite recommend:

   - Client-credentials with secret (simplest; today)
   - mTLS at the token endpoint (cert-as-credential, higher assurance)
   - Direct cert-backed JWT bearer (RFC 7523; the OpenTDF SDK mentions DPoP, which is disabled in TDFLite but exists)

   Will's request: "getting explicit NPE certs should be huge; perhaps even allowing local decrypt of TDF's if they're bound to a TDF platform as well as my local cert (TDF allows for that 'OR' structure nicely)." That last part — **KAS policy with OR-clauses mixing platform-bound + local-cert-bound access** — is a federation story we want to explore; is there precedent in OpenTDF platform ABAC?

5. **Embedded TDFLite as a filedag dependency vs sidecar.** For Will's localhost-first setup (one laptop, one user), options:
   - filedag spawns tdflite as a subprocess on startup (hidden infrastructure)
   - User runs `tdflite up` separately and filedag just connects to `:15433`
   - filedag imports `embedded-postgres` + `idplite` + OpenTDF SDK directly, becomes "its own TDFLite" internally

   Is the import-as-library path blessed, or is sidecar the intended pattern?

6. **Revocation in the zero-trust sense.** Our ABAC plan delegates revocation to KAS (key non-release). When a user's membership changes (add department, revoke clearance), what's the TDFLite flow?
   - Re-seal `policy.sealed.json`, restart tdflite (the README suggests this)
   - Is there an online policy-change API that doesn't require restart?
   - What's the propagation SLA to dependent services that have cached tokens?

7. **Subject condition grammar for custom attributes.** If we define `rating` as a TDFLite hierarchy attribute with values `[x, r, pg, g]` (highest-to-lowest), subject-mapping an identity with `rating: r` → can access `r/pg/g` content. Does the platform ABAC handle the hierarchy comparison, or do we express the rule (`IN_CONTAINS`, `IN`) in subject mappings ourselves? A worked example for a filedag-style hierarchy would be gold.

## Offer in return

filedag will contribute back to TDFLite / OpenTDF ecosystem:

- **Claims-to-memberships translator** as a reusable Go package once we've shipped filedag's impl — if the translator is generic enough, we'd promote to a peer repo (proposed: `github.com/opentdf/claims-translator` or within TDFLite's `internal/` as `idpclaims`).
- **Reference ABAC integration doc** showing a Go service consuming TDFLite end-to-end: from `tdflite up` to validated-JWT to filtered content response. Pairs with the existing README quick-start.
- **NPE cert auth pattern writeup** once Will's requirement lands — content-bound OR-clause KAS policies mixing platform + local-cert keys is a story worth making sharable.

## Timeline / urgency

- filedag's `internal/abac/` Phase 1 ships next session with `ConfigEntitlementSource` (a stub that reads `policy.sealed.json` directly — no OIDC round-trip, just the JSON claims).
- `TDFLiteOIDCEntitlementSource` is Phase 2 (follow-up session). That's when these answers bite.
- No blocking urgency; responses whenever convenient.

## Contact

filedag contact: Will (simultaneous architect). All correspondence via this file or direct peer-repo PR works.

---

**REBAR feedback-loop version:** v1. filedag will drop a follow-up in `~/dev/rebar/feedback/processed/` once integration lands with the answers incorporated.
