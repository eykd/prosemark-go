# Web Security: XSS, CSRF, CSP, Headers

## Table of Contents

- [XSS Prevention](#xss-prevention)
- [CSRF Protection](#csrf-protection)
- [Content Security Policy](#content-security-policy)
- [Security Headers](#security-headers)

## XSS Prevention

### Output Encoding (Primary Defense)

Go's `html/template` package auto-escapes by default:

```go
import "html/template"

// html/template auto-escapes interpolated values in HTML context
tmpl := template.Must(template.New("page").Parse(`
	<div class="user">{{.Name}}</div>
	<a href="{{.ProfileURL}}">Profile</a>
`))

// Values are escaped based on context (HTML body, attribute, URL, etc.)
tmpl.Execute(w, user) // auto-encoded
```

For manual encoding outside templates:

```go
import "html"

// html.EscapeString escapes HTML special characters
safe := html.EscapeString(userInput)
```

### Context-Specific Encoding

| Context        | Encoding Required                           |
| -------------- | ------------------------------------------- |
| HTML body      | `html/template` auto-escapes                |
| HTML attribute | `html/template` auto-escapes in attributes  |
| JavaScript     | `json.Marshal` or `template.JSEscapeString` |
| URL            | `url.QueryEscape` or `url.PathEscape`       |
| CSS            | Avoid user input; allowlist if needed        |

### Flag These as Critical

- `fmt.Fprintf(w, ...)` with user data in HTML (use `html/template`)
- `text/template` for HTML rendering (use `html/template`)
- Direct string concatenation for HTML output
- `template.HTML()` type cast on user input

## CSRF Protection

### Session-Tied CSRF Tokens

```go
import "crypto/rand"

// createSession generates a session with a CSRF token.
func createSession(userID string) (*Session, error) {
	sessionID, err := generateSecureToken(32) // 256 bits
	if err != nil {
		return nil, err
	}
	csrfToken, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}
	return &Session{
		ID:        sessionID,
		UserID:    userID,
		CSRFToken: csrfToken,
	}, nil
}

func generateSecureToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

### CSRF Middleware

```go
import "crypto/subtle"

func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip safe methods
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		session := getSession(r)
		if session == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Extract from header or form
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("_csrf")
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
			http.Error(w, "CSRF validation failed", http.StatusForbidden)
			return
		}

		// Also validate Origin header
		if !validateOrigin(r) {
			http.Error(w, "Invalid origin", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
```

### Origin Validation

```go
import "net/url"

func validateOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	host := r.Host

	if origin == "" {
		referer := r.Header.Get("Referer")
		if referer == "" {
			return true // Allow but log
		}
		u, err := url.Parse(referer)
		if err != nil {
			return false
		}
		return u.Host == host
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == host
}
```

### Flag These as High

- State-changing endpoints without CSRF validation
- CSRF tokens not tied to sessions
- Missing Origin/Referer validation
- GET requests that modify state

## Content Security Policy

### Recommended CSP

```go
func buildCSP(nonce string) string {
	directives := []string{
		"default-src 'self'",
		"script-src 'self'" + nonceDirective(nonce),
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: https:",
		"font-src 'self'",
		"connect-src 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"object-src 'none'",
	}
	return strings.Join(directives, "; ")
}

func nonceDirective(nonce string) string {
	if nonce == "" {
		return ""
	}
	return " 'nonce-" + nonce + "'"
}
```

### CSP Directive Reference

| Directive         | Purpose                                  |
| ----------------- | ---------------------------------------- |
| `default-src`     | Fallback for all fetches                 |
| `script-src`      | JavaScript sources                       |
| `style-src`       | CSS sources                              |
| `connect-src`     | XHR, fetch, WebSocket                    |
| `frame-ancestors` | Who can embed (replaces X-Frame-Options) |
| `form-action`     | Form submission targets                  |
| `base-uri`        | Allowed `<base>` URLs                    |

### Flag These as Medium

- Missing CSP header
- `unsafe-inline` for scripts
- `unsafe-eval` without justification
- Overly permissive `default-src`

## Security Headers

### Required Headers

```go
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// HSTS - force HTTPS
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

		// Prevent MIME sniffing
		h.Set("X-Content-Type-Options", "nosniff")

		// Clickjacking protection
		h.Set("X-Frame-Options", "DENY")

		// Referrer policy
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		next.ServeHTTP(w, r)
	})
}
```

### Header Checklist

| Header                      | Value                                 | Purpose               |
| --------------------------- | ------------------------------------- | --------------------- |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` | Force HTTPS           |
| `X-Content-Type-Options`    | `nosniff`                             | Prevent MIME sniffing |
| `X-Frame-Options`           | `DENY`                                | Prevent clickjacking  |
| `Content-Security-Policy`   | See above                             | Resource restrictions |
| `Referrer-Policy`           | `strict-origin-when-cross-origin`     | Control referrer      |
| `Permissions-Policy`        | `camera=(), microphone=()`            | Disable features      |

### Cache Control for Authenticated Content

```go
// Prevent caching of sensitive responses
h.Set("Cache-Control", "no-store, no-cache, must-revalidate")
h.Set("Pragma", "no-cache")
```

### Flag These as Medium

- Missing HSTS
- Short HSTS max-age (< 1 year)
- Missing X-Content-Type-Options
- X-Frame-Options allowing embedding
- Sensitive content without no-store
