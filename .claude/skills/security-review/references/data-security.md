# Data Security: Injection & Validation

## Table of Contents

- [SQL Injection Prevention](#sql-injection-prevention)
- [Input Validation](#input-validation)
- [Mass Assignment Protection](#mass-assignment-protection)
- [Secrets Management](#secrets-management)

## SQL Injection Prevention

### Parameterized Queries (Required)

```go
// CORRECT - Parameter binding
var user User
err := db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = $1", userID).Scan(&user)

// CORRECT - Multiple parameters
rows, err := db.QueryContext(ctx,
	"SELECT * FROM tasks WHERE user_id = $1 AND status = $2",
	userID, status)

// WRONG - String concatenation
row := db.QueryRow("SELECT * FROM users WHERE id = '" + userID + "'") // WRONG

// WRONG - fmt.Sprintf
row := db.QueryRow(fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email)) // WRONG
```

### Dynamic Query Building

```go
// Safe: Dynamic conditions with parameters
func buildQuery(filters TaskFilters) (string, []any) {
	conditions := []string{"user_id = $1"}
	params := []any{filters.UserID}
	paramIdx := 2

	if filters.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", paramIdx))
		params = append(params, filters.Status)
		paramIdx++
	}

	if !filters.CreatedAfter.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at > $%d", paramIdx))
		params = append(params, filters.CreatedAfter)
		paramIdx++
	}

	limit := filters.Limit
	if limit == 0 {
		limit = 50
	}
	params = append(params, limit)

	query := fmt.Sprintf("SELECT * FROM tasks WHERE %s LIMIT $%d",
		strings.Join(conditions, " AND "), paramIdx)

	return query, params
}
```

### Safe Dynamic Column Names

```go
// Allowlist pattern for column names
var (
	allowedColumns    = map[string]bool{"created_at": true, "updated_at": true, "title": true, "status": true}
	allowedDirections = map[string]bool{"ASC": true, "DESC": true}
)

func buildOrderClause(column, direction string) (string, error) {
	if !allowedColumns[column] {
		return "", fmt.Errorf("invalid column: %s", column)
	}
	dir := strings.ToUpper(direction)
	if !allowedDirections[dir] {
		return "", fmt.Errorf("invalid direction: %s", direction)
	}
	// Safe to interpolate after allowlist validation
	return fmt.Sprintf("ORDER BY %s %s", column, dir), nil
}
```

### Flag These as Critical

- Any string interpolation/concatenation in SQL
- `fmt.Sprintf` with user input in queries
- Dynamic table/column names without allowlist
- Raw SQL execution without parameterization

## Input Validation

### Validation Framework

```go
// ValidationError represents a single field validation failure.
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

// ValidationResult holds the outcome of input validation.
type ValidationResult struct {
	Errors []ValidationError
}

// IsValid reports whether validation passed.
func (r ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// ValidateString checks string constraints.
func ValidateString(field, value string, minLen, maxLen int, required bool) []ValidationError {
	var errs []ValidationError
	trimmed := strings.TrimSpace(value)

	if required && trimmed == "" {
		errs = append(errs, ValidationError{Field: field, Message: "Required", Code: "REQUIRED"})
		return errs
	}
	if minLen > 0 && len(trimmed) < minLen {
		errs = append(errs, ValidationError{
			Field: field, Message: fmt.Sprintf("Min %d chars", minLen), Code: "TOO_SHORT"})
	}
	if maxLen > 0 && len(trimmed) > maxLen {
		errs = append(errs, ValidationError{
			Field: field, Message: fmt.Sprintf("Max %d chars", maxLen), Code: "TOO_LONG"})
	}
	return errs
}
```

### Allowlist Validation

```go
// ValidateAllowlist checks that a value is in an allowed set.
func ValidateAllowlist(field, value string, allowed []string) []ValidationError {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	// Log as security event - shouldn't happen with valid client
	log.Printf("allowlist violation: field=%s value=%s", field, value)
	return []ValidationError{{
		Field: field, Message: "Invalid selection", Code: "NOT_ALLOWED"}}
}

// Usage
var validStatuses = []string{"pending", "active", "completed"}
errs := ValidateAllowlist("status", input.Status, validStatuses)
```

### Email Validation

```go
import "net/mail"

// ValidateEmail validates and normalizes an email address.
func ValidateEmail(field, value string) (string, []ValidationError) {
	if len(value) > 254 {
		return "", []ValidationError{{Field: field, Message: "Max 254 chars", Code: "TOO_LONG"}}
	}

	addr, err := mail.ParseAddress(value)
	if err != nil {
		return "", []ValidationError{{Field: field, Message: "Invalid email format", Code: "INVALID_FORMAT"}}
	}

	// Normalize for storage (prevent duplicate accounts)
	normalized := strings.ToLower(addr.Address)
	return normalized, nil
}
```

### Validation in Handlers

```go
func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result := validateCreateUser(req)
	if !result.IsValid() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(result.Errors)
		return
	}

	// Proceed with validated data
	user, err := createUser(r.Context(), req)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(user)
}
```

### Flag These as High

- Missing server-side validation
- Client-side only validation
- Type coercion without validation
- Missing length limits on strings

## Mass Assignment Protection

### Explicit Field Selection

```go
// WRONG - Updates any field from request
func updateUser(ctx context.Context, id string, data map[string]any) error {
	// Dangerous: client controls which fields are updated
}

// CORRECT - Explicit allowed fields
type UpdateUserRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
	// Note: no Role or IsAdmin fields
}

func updateUser(ctx context.Context, db *sql.DB, id string, req UpdateUserRequest) error {
	var updates []string
	var params []any
	paramIdx := 1

	if req.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", paramIdx))
		params = append(params, *req.Name)
		paramIdx++
	}
	if req.Email != nil {
		updates = append(updates, fmt.Sprintf("email = $%d", paramIdx))
		params = append(params, *req.Email)
		paramIdx++
	}

	if len(updates) == 0 {
		return nil
	}

	params = append(params, id)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d",
		strings.Join(updates, ", "), paramIdx)
	_, err := db.ExecContext(ctx, query, params...)
	return err
}
```

### DTO Pattern

```go
// Define exactly what's accepted
type CreateTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // validated to "pending" | "active"
	// Explicitly omit: UserID, CreatedAt, ID (set server-side)
}

func createTask(ctx context.Context, req CreateTaskRequest, userID string) (*Task, error) {
	return &Task{
		ID:          uuid.NewString(),    // Server-controlled
		UserID:      userID,              // From session
		CreatedAt:   time.Now(),          // Server-controlled
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
	}, nil
}
```

### Flag These as High

- `json.Unmarshal` into domain entities directly
- Dynamic updates from `map[string]any`
- Missing field allowlists

## Secrets Management

### Environment Variables

```go
// CORRECT - Access via environment
apiKey := os.Getenv("API_SECRET")
if apiKey == "" {
	log.Fatal("API_SECRET environment variable is required")
}

// WRONG - Hardcoded secrets
const apiKey = "sk-abc123..."   // WRONG
const dbPassword = "password123" // WRONG
```

### Configuration Pattern

```go
// Config holds application configuration loaded from environment.
type Config struct {
	APISecret    string
	DatabaseURL  string
	JWTSecret    string
	Environment  string
	LogLevel     string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		APISecret:   os.Getenv("API_SECRET"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		Environment: os.Getenv("ENVIRONMENT"),
		LogLevel:    os.Getenv("LOG_LEVEL"),
	}

	// Validate required secrets
	if cfg.APISecret == "" {
		return nil, errors.New("API_SECRET is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}
```

### Flag These as Critical

- Secrets in source code
- Secrets in config files committed to git
- API keys in client-side code
- Secrets in error messages or logs

### Secrets Checklist

- [ ] No hardcoded secrets
- [ ] Secrets in environment variables
- [ ] Secrets not logged
- [ ] Secrets not in error responses
- [ ] Different secrets per environment
- [ ] Secret rotation capability
