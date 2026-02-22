---
name: security-review
description: Review code for security vulnerabilities and best practices. Use when (1) reviewing code for security issues, (2) auditing authentication/session handling, (3) checking for XSS/CSRF/SQL injection vulnerabilities, (4) evaluating security headers and CSP, (5) validating input handling and output encoding, (6) assessing password hashing and secrets management, (7) reviewing rate limiting and brute force protection, or (8) general security hardening of web applications.
---

# Security Review Skill

Systematic security review following OWASP guidelines and defense-in-depth principles.

## Audience: Non-Technical Managers

**CRITICAL**: Write the ENTIRE review for non-technical managers, not developers.

- Technical jargon is OK when explained briefly and concisely
- Always follow technical terms with plain English explanation (e.g., "XSS (cross-site scripting) - injecting malicious code into web pages")
- Explain security risks in business terms (data breach, financial loss, reputation damage)
- Focus on impacts and consequences, not implementation details
- Keep explanations concise - don't over-explain
- Keep all sections accessible to non-technical readers

## Review Process

1. **Identify security surface**: Authentication, data handling, user input, external APIs
2. **Check each domain** using references below
3. **Prioritize findings**: Critical > High > Medium > Low
4. **Provide actionable fixes** with code examples

## Security Domains

### Authentication & Sessions

See [references/auth-security.md](references/auth-security.md) for password hashing (Argon2id), session management (secure cookies), account lockout, constant-time comparisons.

### Web Security (XSS/CSRF/Headers)

See [references/web-security.md](references/web-security.md) for XSS prevention, CSRF tokens, Content Security Policy, security headers.

### Data Security (Injection/Validation)

See [references/data-security.md](references/data-security.md) for SQL injection prevention, input validation, parameterized queries.

### Quick Checklist

See [references/checklist.md](references/checklist.md) for rapid security audit.

## Critical Patterns to Flag

### Always Critical

```go
// SQL Injection - string interpolation
db.QueryRow("SELECT * FROM users WHERE id = '" + userID + "'") // WRONG

// Missing output encoding
fmt.Fprintf(w, "<div>%s</div>", userInput) // WRONG (use html/template)

// Weak password hashing
md5.Sum([]byte(password))    // WRONG
sha256.Sum256([]byte(password)) // WRONG (no salt)

// Hardcoded secrets
const apiKey = "sk-abc123..." // WRONG
```

### Always High

```go
// Missing CSRF on state-changing endpoints
mux.HandleFunc("POST /api/transfer", handler) // WRONG (no CSRF)

// Insecure cookies
http.SetCookie(w, &http.Cookie{Name: "session", Value: id}) // WRONG (missing flags)

// Missing rate limiting on auth
mux.HandleFunc("POST /login", handler) // WRONG

// Timing-vulnerable comparisons
if token == storedToken { // WRONG (use crypto/subtle)
```

## Secure Patterns

### Safe HTML Templating

```go
// html/template auto-escapes by default
tmpl := template.Must(template.New("page").Parse(`<div>{{.Name}}</div>`))
tmpl.Execute(w, user) // auto-encoded
```

### Parameterized Queries

```go
db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = $1", userID) // correct
```

### Secure Cookies

```go
http.SetCookie(w, &http.Cookie{
    Name:     "__Host-session",
    Value:    sessionID,
    Path:     "/",
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteLaxMode,
    MaxAge:   86400,
})
```

### Constant-Time Compare

```go
import "crypto/subtle"

// Use for all security-sensitive comparisons
subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 // correct
```

## Review Output Format

```markdown
## Security Review: [Component]

### Critical Issues

1. **[Issue]** - [File:Line]
   - Problem: [Description in plain English]
   - Risk: [Business impact - data breach, financial loss, etc.]
   - Fix: [What needs to be done]

### High Priority

...

### Recommendations

...

---

## Copy-Paste Prompt for Claude Code

**REQUIRED when findings exist**: Provide a ready-to-use prompt in a code block.
```

[Specific, actionable prompt with file paths and line numbers that addresses all Critical and High Priority items]

```
```

## Go net/http Security

### Middleware Pattern

```go
// securityHeaders wraps a handler to add standard security headers.
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        next.ServeHTTP(w, r)
    })
}
```

### CSRF Middleware

```go
func csrfMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip safe methods
        if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
            next.ServeHTTP(w, r)
            return
        }
        token := r.Header.Get("X-CSRF-Token")
        session := getSession(r)
        if session == nil || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
            http.Error(w, "CSRF validation failed", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```
