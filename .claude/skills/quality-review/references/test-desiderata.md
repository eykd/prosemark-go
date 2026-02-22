# Test Quality Evaluation Reference

Guidance for evaluating test quality in pull request reviews.

---

## When to Apply

Evaluate test quality when the PR includes test files:

- `*_test.go` (Go)
- Files in `test/`, `testdata/` directories

---

## Kent Beck's Test Desiderata

Evaluate tests against these 12 properties:

### Core Properties

| Property          | Question                          | Signs of Violation                             |
| ----------------- | --------------------------------- | ---------------------------------------------- |
| **Isolated**      | Can tests run independently?      | Shared state, test order dependencies          |
| **Composable**    | Can tests run in any combination? | Global setup/teardown affecting others         |
| **Deterministic** | Same result every run?            | Time-based, random data, external dependencies |
| **Fast**          | Quick feedback?                   | I/O operations, network calls, large datasets  |

### Structural Properties

| Property                  | Question                            | Signs of Violation                            |
| ------------------------- | ----------------------------------- | --------------------------------------------- |
| **Writable**              | Easy to add new tests?              | Copy-paste boilerplate, complex setup         |
| **Readable**              | Clear what's being tested?          | Magic numbers, unclear assertions, long tests |
| **Behavioral**            | Tests outcomes, not implementation? | Testing unexported methods, mocking internals |
| **Structure-insensitive** | Survive refactoring?                | Tightly coupled to implementation details     |

### Meta Properties

| Property       | Question               | Signs of Violation                           |
| -------------- | ---------------------- | -------------------------------------------- |
| **Automated**  | No manual steps?       | Manual assertions, visual inspection needed  |
| **Specific**   | Clear failure message? | Generic assertions, unclear failure location |
| **Predictive** | Catches real bugs?     | Tests only happy path, missing edge cases    |
| **Inspiring**  | Confidence to deploy?  | Low coverage, missing critical paths         |

---

## GOOS Principles

From "Growing Object-Oriented Software, Guided by Tests":

### Test Classification

| Type            | Purpose                   | Characteristics        |
| --------------- | ------------------------- | ---------------------- |
| **Unit**        | Single component behavior | Fast, isolated, no I/O |
| **Integration** | Component collaboration   | Tests real boundaries  |
| **Acceptance**  | User-visible behavior     | End-to-end, slower     |

### Mocking Guidelines

**Mock Roles, Not Objects**

```go
// Good: Mock the role (interface)
type mockNotifier struct {
	notifiedOrders []Order
}

func (m *mockNotifier) NotifyShipped(order Order) error {
	m.notifiedOrders = append(m.notifiedOrders, order)
	return nil
}

// Test uses the interface
service := NewOrderService(&mockNotifier{})
service.ShipOrder(order)
```

**Don't Mock Values**

```go
// Good: Use real value objects
money, _ := NewMoney(10000, "USD")
if !cart.Total().Equal(money) {
	t.Errorf("want %v, got %v", money, cart.Total())
}

// Bad: Don't create mock value objects -- they're pure data
```

**Verify Interactions, Not State**

```go
// Good: Verify the right call was made
if len(repo.savedTasks) != 1 {
	t.Fatal("expected task to be saved")
}

// Less ideal: Check internal state
```

---

## Test Anti-Patterns to Flag

### Critical Anti-Patterns

| Anti-Pattern               | Description                         | Fix                                |
| -------------------------- | ----------------------------------- | ---------------------------------- |
| **Missing Assertions**     | Test runs but verifies nothing      | Add meaningful assertions          |
| **Testing Framework Code** | Testing library/framework internals | Test your code, not theirs         |
| **Shared Mutable State**   | Tests affect each other             | Isolate setup, use fresh instances |
| **Production Data**        | Tests depend on real data           | Use fixtures or factories          |

### High Priority Anti-Patterns

| Anti-Pattern          | Description                          | Fix                           |
| --------------------- | ------------------------------------ | ----------------------------- |
| **Test-Per-Method**   | Mapping 1:1 with implementation      | Test behaviors, not methods   |
| **Excessive Mocking** | More mocks than real objects         | Simplify design, use fakes    |
| **Logic in Tests**    | Conditionals, loops in test code     | Keep tests linear and obvious |
| **Obscure Test**      | Can't understand what's being tested | Improve naming, simplify      |

### Medium Priority Anti-Patterns

| Anti-Pattern                | Description                    | Fix                           |
| --------------------------- | ------------------------------ | ----------------------------- |
| **Test Doubles Everywhere** | Never testing real integration | Add focused integration tests |
| **Assertion Roulette**      | Multiple unrelated assertions  | One concept per test          |
| **Long Tests**              | Hard to understand at a glance | Extract setup, split tests    |
| **Eager Test**              | Testing too many things        | Focus on one behavior         |

---

## Test Pain as Design Feedback

When tests are hard to write, it often indicates design problems:

| Test Difficulty             | Possible Design Issue             |
| --------------------------- | --------------------------------- |
| **Complex setup**           | Object has too many dependencies  |
| **Hard to mock**            | Interface too large, violates ISP |
| **Brittle tests**           | Implementation details exposed    |
| **Slow tests**              | Missing abstraction boundaries    |
| **Can't test in isolation** | Tight coupling between components |

---

## Coverage Verification

**CRITICAL**: This project requires 100% test coverage of non-Impl functions.

When reviewing test changes, ALWAYS verify coverage:

```bash
just test-cover-check
```

**Coverage policy**:

- All non-Impl functions must be 100% covered
- `*Impl` functions (wrapping exec.Command, os operations) are exempt
- The coverage check script filters out Impl functions from calculation

### Finding Format for Insufficient Coverage

**If coverage is below 100%**, report as HIGH severity:

```markdown
### Finding: Test coverage below 100%

- **Severity**: High
- **Category**: test
- **File**: path/to/file.go
- **Description**: Coverage is at X%. Project requires 100% coverage for non-Impl functions.
- **Impact**: Pre-commit hooks will fail. Untested code paths may contain bugs.
- **Fix**: Add tests for uncovered lines. Run `just test-cover` to see gaps.
```

---

## Evaluation Output Format

For the Test Quality section of the review:

```markdown
### Test Quality

#### Classification Assessment

- **Unit Tests**: [Present/Missing] - [Assessment]
- **Integration Tests**: [Present/Missing] - [Assessment]
- **Acceptance Tests**: [Present/Missing] - [Assessment]

#### Test Desiderata Check

| Property      | Status    | Notes                  |
| ------------- | --------- | ---------------------- |
| Isolated      | Pass/Fail | [Specific observation] |
| Deterministic | Pass/Fail | [Specific observation] |
| Fast          | Pass/Fail | [Specific observation] |
| Readable      | Pass/Fail | [Specific observation] |
| Behavioral    | Pass/Fail | [Specific observation] |
| Specific      | Pass/Fail | [Specific observation] |

#### Coverage Report

- **Status**: [Pass/FAIL - see findings]
- **Non-Impl Coverage**: X%

#### Mocking Assessment

- **Pattern**: [Appropriate/Over-mocked/Under-mocked]
- **Observations**: [Specific feedback on mocking usage]

#### Anti-Patterns Detected

[List any anti-patterns found, or "None detected"]

#### Design Feedback

[Any observations about design issues indicated by test difficulty, or "No design concerns"]
```

---

## Quick Checklist

For rapid evaluation, verify these essentials:

- [ ] Tests are behavioral (test what, not how)
- [ ] Tests are readable and self-documenting
- [ ] Tests are fast (no unnecessary I/O)
- [ ] Tests are deterministic (no flaky tests)
- [ ] Tests are isolated (no shared state)
- [ ] Tests are specific (clear failure messages)
- [ ] Mocking is appropriate (roles, not values)
- [ ] No critical anti-patterns present
- [ ] Coverage is 100% for non-Impl functions
- [ ] Test pain signals are addressed or acknowledged

---

## Examples

### Good Test Pattern

```go
func TestOrderService_ShipOrder(t *testing.T) {
	t.Run("notifies customer when order ships", func(t *testing.T) {
		// Arrange
		notifier := &spyNotifier{}
		order := newTestOrder(withStatus(StatusPacked))
		service := NewOrderService(notifier)

		// Act
		err := service.ShipOrder(order)

		// Assert
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(notifier.shipped) != 1 {
			t.Fatal("expected notification to be sent")
		}
		if order.Status() != StatusShipped {
			t.Errorf("want status %v, got %v", StatusShipped, order.Status())
		}
	})
}
```

**Why it's good**:

- Clear behavior being tested (in the name)
- Minimal setup
- Tests role interaction (notifier)
- Verifiable outcome
- Follows AAA pattern (Arrange-Act-Assert)

### Bad Test Pattern

```go
var testDB *sql.DB // Shared state!

func TestMain(m *testing.M) {
	testDB = connectRealDB() // Real I/O in test setup
	os.Exit(m.Run())
}

func TestOrderService(t *testing.T) {
	service := NewOrderService(testDB)
	result, _ := service.Process(nil) // What's being tested?
	if result == nil {                 // Weak assertion
		t.Fatal("expected result")
	}
}
```

**Why it's bad**:

- Shared mutable state (package-level testDB)
- Unclear what behavior is tested
- No isolation (real DB)
- Weak assertion (just nil check)
- Ignored error return
