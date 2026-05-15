📍 **You are here:** [Try It](QUICKSTART.md) → **Love It (1 hour)** → [Master It](CASE-STUDIES.md)
**Prerequisites:** [QUICKSTART.md](QUICKSTART.md) complete
**Next step:** [Agent coordination](agents/README.md) or [real-world patterns](CASE-STUDIES.md)

# Feature Development Guide

**The rebar way to build features: BDD → Contract → Code → Test → Agent Review**

This 1-hour guided journey shows you rebar's full value by walking through a complete feature from conception to production. You'll experience both sides of rebar: the **information organization model** (how to structure knowledge) and the **swarm coordination platform** (how agents work together).

---

## Prerequisites

- Basic rebar setup complete ([QUICKSTART.md](QUICKSTART.md) if needed)
- A codebase with at least one existing contract
- Access to Claude Code or similar AI coding assistant

---

## The Journey: User Authentication Feature

We'll build a user authentication system to demonstrate the full rebar workflow. This example shows real patterns you'll use for any feature.

### Step 1: Start with BDD (5 minutes)
*Define what success looks like before writing any code*

**Create the scenario:**
```gherkin
# features/user-authentication.feature

Feature: User Authentication
  As a web application
  I want to authenticate users securely
  So that I can protect user data and personalize experiences

Scenario: Successful login
  Given a user "alice@example.com" with password "secure123"
  When the user submits login credentials
  Then they receive a JWT token valid for 24 hours
  And their session is tracked in the database

Scenario: Failed login
  Given invalid credentials are submitted
  When the user attempts to login
  Then they receive an "Invalid credentials" error
  And no session is created
  And the attempt is logged for security monitoring

Scenario: Token validation
  Given a valid JWT token
  When the token is verified
  Then user identity is confirmed
  And access to protected resources is granted
```

**Why BDD first:** Agents reading this know exactly what behavior to implement. No ambiguity about edge cases, error handling, or success criteria.

### Step 2: Design the Contract (10 minutes)
*Specify behavior that multiple agents can implement consistently*

**Create the contract:**
```bash
# Create the contract file
touch architecture/CONTRACT-S3-AUTH.1.0.md
```

```markdown
# CONTRACT: S3-AUTH.1.0
**Authentication Service Contract**

## Overview
Stateless authentication service providing JWT-based user authentication with session tracking.

## Interface

### `Authenticate(email, password) -> (token, error)`
**Purpose:** Validate user credentials and issue access token

| Input | Type | Validation |
|-------|------|------------|
| email | string | Valid email format, max 254 chars |
| password | string | 8-72 characters |

| Outcome | Return | Side Effects |
|---------|--------|--------------|
| Valid credentials | JWT token (24h expiry) | Session record created |
| Invalid credentials | `ErrInvalidCredentials` | Attempt logged, no session |
| Account locked | `ErrAccountLocked` | Security event logged |

### `ValidateToken(token) -> (userID, error)`
**Purpose:** Verify JWT token and extract user identity

| Input | Type | Validation |
|-------|------|------------|
| token | string | Valid JWT format |

| Outcome | Return | Side Effects |
|---------|--------|--------------|
| Valid token | User ID (UUID) | Access granted |
| Expired token | `ErrTokenExpired` | None |
| Invalid token | `ErrInvalidToken` | Security event logged |

## Dependencies
- **Database**: User credentials, session storage
- **Logging**: Security events, failed attempts
- **Config**: JWT secret, token expiry settings

## Security Requirements
- Passwords hashed with bcrypt (cost 12+)
- JWT tokens signed with RS256
- Rate limiting: 5 attempts per email per 15 minutes
- Session cleanup: Remove expired sessions daily

## Testing Requirements
- **T0**: Unit tests for each public method
- **T1**: Integration tests with test database
- **T2**: Security tests (timing attacks, malformed inputs)
- **T3**: Load testing with realistic user patterns

## Implementation Notes
- Token claims: `{userID, email, exp, iat}`
- Session table: `{id, userID, tokenID, createdAt, expiresAt}`
- Log format: `{timestamp, event, userEmail, outcome, metadata}`
```

**Link to code:**
```go
// auth/service.go
// CONTRACT:S3-AUTH.1.0
package auth
```

**Why contracts matter:** Any agent can now implement this service correctly. They know the exact error types, security requirements, and testing expectations.

### Step 3: Agent Coordination (15 minutes)
*Get architecture, product, and engineering perspectives before coding*

**Ask the architect:**
```bash
ask architect "Review CONTRACT:S3-AUTH.1.0 - does this fit our security model and integrate cleanly with existing services?"
```

*Sample response:*
> ✅ **Architecture Review: S3-AUTH.1.0**
>
> **Fits well:** JWT stateless design aligns with microservice architecture
> **Integration:** Clean dependency on existing user store and logging service
> **Recommendation:** Add refresh token mechanism for long-lived sessions
> **Security:** bcrypt + RS256 is appropriate, consider adding 2FA hooks for future

**Ask product:**
```bash
ask product "Does S3-AUTH.1.0 meet our user experience requirements? Any missing scenarios?"
```

*Sample response:*
> ✅ **Product Review: S3-AUTH.1.0**
>
> **UX Coverage:** Login/logout flow covered
> **Missing:** Password reset, account lockout recovery
> **Priority:** Core auth is P0, recovery flows are P1
> **Recommendation:** Ship core contract, add password reset as S3-AUTH.1.1

**Ask engineering lead:**
```bash
ask englead "Review S3-AUTH.1.0 testing requirements - are T0-T3 tests sufficient for production confidence?"
```

*Sample response:*
> ✅ **Engineering Review: S3-AUTH.1.0**
>
> **Testing:** T0-T3 appropriate for auth service
> **Add:** T4 integration tests with rate limiting
> **Security:** Include timing attack tests in T2
> **Deployment:** Recommend feature flag for gradual rollout

**Why multi-agent review:** Each role sees different risks and opportunities. Architect catches integration issues, product ensures user needs are met, eng lead validates delivery approach.

### Step 4: Implementation (20 minutes)
*Write code that fulfills the contract*

**Core service implementation:**
```go
// auth/service.go
// CONTRACT:S3-AUTH.1.0
package auth

import (
    "time"
    "github.com/golang-jwt/jwt/v4"
    "golang.org/x/crypto/bcrypt"
)

type Service struct {
    userStore UserStore
    logger    Logger
    jwtSecret []byte
    tokenTTL  time.Duration
}

func (s *Service) Authenticate(email, password string) (string, error) {
    // Contract validation: email format, password length
    if !isValidEmail(email) || len(password) < 8 || len(password) > 72 {
        return "", ErrInvalidCredentials
    }

    // Rate limiting check
    if s.isRateLimited(email) {
        s.logger.SecurityEvent("rate_limit_exceeded", email, nil)
        return "", ErrRateLimited
    }

    // Fetch user and verify password
    user, err := s.userStore.GetByEmail(email)
    if err != nil {
        s.logger.SecurityEvent("login_attempt", email, map[string]interface{}{
            "outcome": "user_not_found",
        })
        return "", ErrInvalidCredentials
    }

    if !s.verifyPassword(password, user.PasswordHash) {
        s.logger.SecurityEvent("login_attempt", email, map[string]interface{}{
            "outcome": "invalid_password",
        })
        return "", ErrInvalidCredentials
    }

    // Generate JWT token
    token, err := s.generateToken(user.ID, email)
    if err != nil {
        return "", err
    }

    // Create session record
    if err := s.createSession(user.ID, token); err != nil {
        return "", err
    }

    s.logger.SecurityEvent("login_success", email, map[string]interface{}{
        "userID": user.ID,
    })

    return token, nil
}

func (s *Service) ValidateToken(tokenString string) (string, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        return s.jwtSecret, nil
    })

    if err != nil {
        s.logger.SecurityEvent("token_validation", "", map[string]interface{}{
            "outcome": "invalid_token",
            "error": err.Error(),
        })
        return "", ErrInvalidToken
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok || !token.Valid {
        return "", ErrInvalidToken
    }

    userID := claims["userID"].(string)
    return userID, nil
}
```

**Why implementation follows contract:** Every behavior specified in the contract is implemented exactly. Error types, logging, validation rules all match the specification.

### Step 5: Testing Cascade (10 minutes)
*Validate each tier of the testing pyramid*

**T0 - Unit Tests:**
```go
// auth/service_test.go
// CONTRACT:S3-AUTH.1.0
func TestAuthenticate_ValidCredentials(t *testing.T) {
    service := setupTestService()

    token, err := service.Authenticate("alice@example.com", "secure123")

    assert.NoError(t, err)
    assert.NotEmpty(t, token)
    // Verify JWT claims...
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
    service := setupTestService()

    token, err := service.Authenticate("alice@example.com", "wrong")

    assert.Equal(t, ErrInvalidCredentials, err)
    assert.Empty(t, token)
}
```

**T1 - Integration Tests:**
```go
func TestAuthenticate_Integration(t *testing.T) {
    db := setupTestDB()
    service := auth.New(db, testLogger, testJWTSecret)

    // Create test user
    user := createTestUser(db, "alice@example.com", "secure123")

    token, err := service.Authenticate("alice@example.com", "secure123")

    assert.NoError(t, err)
    // Verify session was created in database
    sessions := getSessionsForUser(db, user.ID)
    assert.Len(t, sessions, 1)
}
```

**T2 - Security Tests:**
```go
func TestAuthenticate_TimingAttack(t *testing.T) {
    service := setupTestService()

    // Time valid user with wrong password
    start1 := time.Now()
    service.Authenticate("real@example.com", "wrong")
    duration1 := time.Since(start1)

    // Time invalid user
    start2 := time.Now()
    service.Authenticate("fake@example.com", "wrong")
    duration2 := time.Since(start2)

    // Timing should be similar (within 10ms)
    diff := abs(duration1 - duration2)
    assert.Less(t, diff, 10*time.Millisecond)
}
```

**Run the cascade:**
```bash
# T0-T2 run in sequence
go test ./auth/... -v

# Check enforcement
scripts/check-contract-headers.sh  # Verify CONTRACT: headers
scripts/check-todos.sh            # No untracked TODOs
```

**Why testing cascade matters:** Each tier catches different classes of bugs. T0 catches logic errors, T1 catches integration issues, T2 catches security vulnerabilities.

### Step 6: Agent Review & Integration (10 minutes)
*Get automated quality assessment and integration guidance*

**Quality scan:**
```bash
ask steward "Assess CONTRACT:S3-AUTH.1.0 implementation"
```

*Sample response:*
> ✅ **Steward Assessment: S3-AUTH.1.0**
>
> **Contract Status:** ACTIVE (implementation complete)
> **Test Coverage:** T0-T2 complete, T3 pending
> **Dependencies:** 2/3 contracts implemented (UserStore ✅, Logger ✅, Config ⚠️)
> **Issues:** Config contract needs version bump for JWT settings
> **Recommendation:** Ready for integration, schedule T3 load tests

**Integration check:**
```bash
ask merger "Plan integration of S3-AUTH.1.0 with main branch"
```

*Sample response:*
> ✅ **Integration Plan: S3-AUTH.1.0**
>
> **Conflicts:** None detected
> **Dependencies:** Update CONFIG contract to include JWT settings
> **Deployment:** Feature flag recommended for gradual rollout
> **Testing:** Run full T0-T4 suite after merge
> **Rollback:** Previous auth still available via feature flag

**Final commit:**
```bash
git add auth/
git commit -m "feat: implement S3-AUTH.1.0 user authentication service

- JWT-based stateless authentication
- bcrypt password hashing (cost 12)
- Rate limiting: 5 attempts/15min per email
- Session tracking and security logging
- Full T0-T2 test coverage

CONTRACT: S3-AUTH.1.0 → ACTIVE
Closes: #auth-service-implementation"
```

---

## What You Just Experienced

### Information Organization Model ✅
- **BDD scenarios** defined success criteria
- **Contract** specified exact behavior
- **Testing tiers** validated quality systematically
- **Documentation** stayed synchronized with code

### Swarm Coordination Platform ✅
- **Multiple agent perspectives** (architect, product, eng lead)
- **Automated quality assessment** (steward)
- **Integration coordination** (merger)
- **Persistent agent memory** (ASK CLI sessions)

### The Rebar Difference

**Without rebar:**
- Feature requirements live in Slack/email
- Implementation details in developer's head
- Testing is ad-hoc
- Agents work in isolation, duplicate effort

**With rebar:**
- Requirements captured in executable BDD
- Behavior specified in contracts that agents can read
- Testing follows proven cascade (T0-T5)
- Agents coordinate through shared contracts and persistent sessions

---

## Next Steps

### Ready to scale?
- **[Multi-agent orchestration](practices/multi-agent-orchestration.md)** - Fan out implementation across multiple agents
- **[Worktree collaboration](practices/worktree-collaboration.md)** - Parallel development without merge conflicts

### Need specialized patterns?
- **[Contract versioning](architecture/README.md#versioning)** - Breaking changes and upgrade paths
- **[Cross-repo coordination](CASE-STUDIES.md)** - Contract namespacing and shared dependencies

### Want to see war stories?
- **[OpenDocKit case study](feedback/processed/2026-03-18-opendockit-fidelity-session.md)** - 9 simultaneous agents, 5,824 tests
- **[Human-based Digital Signer case study](feedback/processed/digital-signer-feedback.md)** - 18 agents, 0 merge conflicts, 3 hours wall clock

**You've just experienced the full rebar methodology. Every feature you build from now on follows this same pattern: BDD → Contract → Multi-agent coordination → Implementation → Quality cascade → Integration.**

The system scales from solo development to coordinated swarms. The patterns you just learned work whether you're one developer or ten agents working in parallel.