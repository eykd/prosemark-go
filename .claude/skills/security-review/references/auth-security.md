# Authentication & Session Security

## Table of Contents

- [Password Hashing](#password-hashing)
- [Session Management](#session-management)
- [Brute Force Protection](#brute-force-protection)
- [Timing Attack Prevention](#timing-attack-prevention)

## Password Hashing

### Required: Argon2id

OWASP 2025 recommendation. Protects against GPU and side-channel attacks.

```go
import "golang.org/x/crypto/argon2"

type argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var defaultParams = argon2Params{
	Memory:      19456, // 19 MiB minimum
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, defaultParams.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password), salt,
		defaultParams.Iterations, defaultParams.Memory,
		defaultParams.Parallelism, defaultParams.KeyLength,
	)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		defaultParams.Memory, defaultParams.Iterations,
		defaultParams.Parallelism, saltB64, hashB64), nil
}
```

### Flag These as Critical

- `md5`, `sha1`, `sha256` without salt
- `bcrypt` (use Argon2id instead)
- Plain text passwords
- Reversible encryption for passwords

### Password Requirements

- Minimum 12 characters (NIST 800-63B)
- Maximum 128 characters
- Check against common password list
- No complexity requirements (per NIST)

## Session Management

### Secure Cookie Pattern

```go
http.SetCookie(w, &http.Cookie{
	Name:     "__Host-session",  // __Host- prefix required
	Value:    sessionID,
	Path:     "/",               // Required for __Host-
	HttpOnly: true,              // No JavaScript access
	Secure:   true,              // HTTPS only
	SameSite: http.SameSiteLaxMode, // CSRF protection
	MaxAge:   sessionTTLSeconds,
})
```

### Session ID Requirements

- 256+ bits cryptographic randomness
- Generated server-side only
- Regenerate on privilege changes (login, role change)

```go
import "crypto/rand"

func generateSessionID() (string, error) {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating session ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}
```

### Session Storage

```go
// Store session with TTL for automatic expiration
type SessionStore interface {
	Get(ctx context.Context, id string) (*Session, error)
	Put(ctx context.Context, session *Session) error
	Delete(ctx context.Context, id string) error
}

type Session struct {
	ID        string
	UserID    string
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}
```

### Flag These as High

- Session IDs in URLs
- Missing `HttpOnly` flag
- Missing `Secure` flag
- `SameSite=None` without justification
- Long session lifetimes without refresh

## Brute Force Protection

### Account Lockout

```go
const (
	maxFailedAttempts = 5
	lockDuration      = 15 * time.Minute
)

// User tracks authentication state.
type User struct {
	// ...
	failedLoginAttempts int
	lockedUntil         time.Time
}

// IsLocked reports whether the account is currently locked.
func (u *User) IsLocked(now time.Time) bool {
	return !u.lockedUntil.IsZero() && now.Before(u.lockedUntil)
}

// RecordFailedLogin increments the failure count and locks if threshold exceeded.
func (u *User) RecordFailedLogin(now time.Time) {
	u.failedLoginAttempts++
	if u.failedLoginAttempts >= maxFailedAttempts {
		u.lockedUntil = now.Add(lockDuration)
	}
}
```

### Rate Limiting (Sliding Window)

```go
// RateLimitResult holds the outcome of a rate limit check.
type RateLimitResult struct {
	Allowed   bool
	Remaining int
	ResetAt   time.Time
}

// RateLimiter checks request rates per key.
type RateLimiter interface {
	Check(ctx context.Context, key string, limit int, window time.Duration) (RateLimitResult, error)
}

// Apply to auth endpoints
// ipResult, _ := limiter.Check(ctx, "ip:"+clientIP, 10, time.Minute)
// accountResult, _ := limiter.Check(ctx, "account:"+email, 5, 5*time.Minute)
```

### Flag These as High

- Auth endpoints without rate limiting
- No account lockout after failed attempts
- Missing IP-based rate limiting
- Lockout bypass via password reset

## Timing Attack Prevention

### Constant-Time Comparison

```go
import "crypto/subtle"

// Use for all security-sensitive comparisons
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
```

### Use For

- Password verification
- CSRF token validation
- API key comparison
- Session ID comparison
- Any security-sensitive string comparison

### Flag These as High

- `==` for tokens/secrets
- Early return on mismatch
- `strings.Compare()` for security values

## Account Enumeration Prevention

### Generic Error Messages

```go
// WRONG - reveals valid accounts
if user == nil {
	return errors.New("user not found")
}
if !validPassword {
	return errors.New("invalid password")
}

// CORRECT - generic message
if user == nil || !validPassword {
	return errors.New("invalid email or password")
}
```

### Consistent Timing

```go
func login(ctx context.Context, email, password string) error {
	user, err := findUser(ctx, email)

	// Always hash even if user not found (prevents timing leak)
	dummyHash := "$argon2id$v=19$m=19456,t=2,p=1$..."
	hashToVerify := dummyHash
	if err == nil && user != nil {
		hashToVerify = user.PasswordHash
	}

	valid := verifyPassword(password, hashToVerify)

	if user == nil || !valid {
		return errors.New("invalid email or password")
	}
	return nil
}
```
