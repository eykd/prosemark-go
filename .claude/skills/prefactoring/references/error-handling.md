# Error Handling Strategies

**Purpose**: Apply prefactoring principles for robust error handling, meaningful messages, and resilient systems.

## When to Use

- Designing error handling strategies
- Implementing error checks
- Creating user-facing error messages
- Building resilient external integrations

## Core Principles

### Never Be Silent

Every error must be reported. Never discard errors without handling.

```go
// Bad: Silent failure
_ = sendNotification(user)

// Good: Always report
if err := sendNotification(user); err != nil {
    logger.Error("notification failed", "userID", user.ID, "error", err)
    return fmt.Errorf("sending notification for user %s: %w", user.ID, err)
}
```

### Report Meaningful User Messages

Error messages describe what users can do, not technical details.

```go
// Bad: Technical error exposed
return errors.New("SQLITE_CONSTRAINT: UNIQUE constraint failed")

// Good: User-actionable message with wrapped technical detail
type UserFacingError struct {
    Message string
    Code    string
    Cause   error
}

func (e *UserFacingError) Error() string { return e.Message }
func (e *UserFacingError) Unwrap() error { return e.Cause }

return &UserFacingError{
    Message: "An account with this email already exists. Please sign in or use a different email.",
    Code:    "EMAIL_EXISTS",
    Cause:   originalErr,
}
```

### Consider Failure an Expectation

Design for failure with retries, fallbacks, and graceful degradation.

```go
type ResilientNotificationService struct {
    maxAttempts int
    queue       MessageQueue
}

func (s *ResilientNotificationService) Notify(user User, message Message) error {
    // Retry with backoff
    for attempt := 1; attempt <= s.maxAttempts; attempt++ {
        if err := s.tryNotify(user, message); err == nil {
            return nil
        }
        s.backoff(attempt)
    }

    // Fallback
    return s.queueForLaterDelivery(user, message)
}
```

## (T, error) Return Pattern

Use Go's multiple return values to make errors explicit at the call site.

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func ParseEmail(input string) (Email, error) {
    if !strings.Contains(input, "@") {
        return Email{}, &ValidationError{Field: "email", Message: "invalid email"}
    }
    return Email{value: input}, nil
}

// Usage forces error handling
email, err := ParseEmail(input)
if err != nil {
    return fmt.Errorf("parsing email: %w", err)
}
// email is safe to use
```

## Decision Matrix

| Situation           | Apply                  | Example                           |
| ------------------- | ---------------------- | --------------------------------- |
| Error return        | Never Be Silent        | Check and wrap or handle          |
| User-facing error   | Meaningful Messages    | Explain what user can do          |
| External dependency | Failure as Expectation | Add retry/fallback                |
| Function can fail   | (T, error) Returns    | Return value and error explicitly |

## Related References

- [contracts.md](./contracts.md): Validation and interface contracts
- [value-objects.md](./value-objects.md): Domain types with validation
