# Security Review Checklist

Quick reference for security audits. Check each item and flag issues by severity.

## Transport Security

- [ ] HTTPS enforced (redirect HTTP → HTTPS)
- [ ] HSTS header with 2+ year max-age
- [ ] TLS 1.2+ required (prefer 1.3)
- [ ] No mixed content

## Password Security

- [ ] Argon2id hashing (not bcrypt/MD5/SHA)
- [ ] Minimum 12 character passwords
- [ ] Common password check
- [ ] Constant-time password comparison
- [ ] No plain text storage

## Session Security

- [ ] 256+ bit cryptographic session IDs
- [ ] `__Host-` cookie prefix
- [ ] `HttpOnly` flag set
- [ ] `Secure` flag set
- [ ] `SameSite=Lax` or `Strict`
- [ ] Server-side session storage with TTL
- [ ] Session regeneration on privilege change

## CSRF Protection

- [ ] CSRF tokens on all state-changing requests
- [ ] Tokens tied to user sessions
- [ ] Tokens validated server-side
- [ ] Origin/Referer header validation
- [ ] No GET requests that modify state

## XSS Prevention

- [ ] Output encoding by default (html`` template)
- [ ] HTMX `selfRequestsOnly: true`
- [ ] HTMX `allowScriptTags: false`
- [ ] `hx-disable` on user content zones
- [ ] CSP header configured

## Input Validation

- [ ] Server-side validation for all inputs
- [ ] Allowlist validation for fixed options
- [ ] Type checking before use
- [ ] Length limits on strings
- [ ] Email format validation

## Database Security

- [ ] Parameterized queries only
- [ ] No string interpolation in SQL
- [ ] Allowlist for dynamic columns
- [ ] Mass assignment prevention

## Rate Limiting

- [ ] Login attempts limited per IP
- [ ] Login attempts limited per account
- [ ] Account lockout after failures
- [ ] Registration rate limited
- [ ] Password reset rate limited

## Security Headers

- [ ] `Strict-Transport-Security`
- [ ] `Content-Security-Policy`
- [ ] `X-Frame-Options: DENY`
- [ ] `X-Content-Type-Options: nosniff`
- [ ] `Referrer-Policy`
- [ ] `Cache-Control: no-store` for authenticated content

## Secrets Management

- [ ] No hardcoded secrets
- [ ] Secrets in environment variables
- [ ] Secrets not in logs
- [ ] Secrets not in error messages
- [ ] Different secrets per environment

## Error Handling

- [ ] Generic error messages (no stack traces)
- [ ] No database errors exposed
- [ ] No path disclosure
- [ ] Fail secure (deny on error)

## Brute Force Protection

- [ ] Constant-time comparisons
- [ ] Account enumeration prevention
- [ ] Progressive delays/lockouts
- [ ] Generic auth error messages

## Audit & Monitoring

- [ ] Security events logged
- [ ] Failed auth attempts tracked
- [ ] Account lockouts monitored
- [ ] Anomaly detection possible

---

## Severity Guide

### Critical (Fix Immediately)

- SQL injection
- Missing output encoding (XSS)
- Weak password hashing (MD5/SHA/plain)
- Hardcoded secrets
- Authentication bypass

### High (Fix Before Deploy)

- Missing CSRF protection
- Insecure session cookies
- Missing rate limiting on auth
- Timing-vulnerable comparisons
- Mass assignment vulnerabilities

### Medium (Fix Soon)

- Missing security headers
- Weak CSP
- Short HSTS max-age
- Missing input validation
- Verbose error messages

### Low (Track for Fix)

- Missing audit logging
- Suboptimal crypto parameters
- Minor header configurations
