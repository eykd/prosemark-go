---
name: go-property-testing
description: Write property-based tests in Go using the rapid library (pgregory.net/rapid). Use when writing tests for Go code that would benefit from random input generation and automatic shrinking — especially for serialization round-trips, data structure invariants, algebraic laws, parser robustness, or stateful system testing. Also use when the user mentions property-based testing, PBT, QuickCheck-style testing, or rapid in a Go context. Do NOT use for example-based unit tests, integration tests with external services, or performance benchmarks.
---

# Property-Based Testing in Go with `rapid`

## When to Use PBT

Best suited for:
- Round-trip encode/decode (JSON, protobuf, DB serialization)
- Invariant preservation (sort preserves length/elements, BST ordering)
- Algebraic laws (commutativity, associativity, idempotency)
- Oracle comparison (optimized impl vs. reference impl)
- Robustness / "never crashes" for parsers, protocol handlers
- Stateful system testing (sequences of operations against a model)

Not suited for: specific business rules with exact expected outputs, integration tests with external services, benchmarks.

## Quick Start with `rapid`

```bash
go get pgregory.net/rapid
```

```go
rapid.Check(t, func(t *rapid.T) {
    a := rapid.Int().Draw(t, "a")
    b := rapid.Int().Draw(t, "b")
    if Add(a, b) != Add(b, a) {
        t.Fatalf("commutativity violated: Add(%d, %d) != Add(%d, %d)", a, b, b, a)
    }
})
```

Key concepts:
- Draw values with typed generators: `rapid.Int().Draw(t, "label")`
- Always label draws descriptively for readable shrunk output
- Shrinking is automatic and integrated — no separate shrinker needed
- Default: 100 iterations. Override: `rapid.Checks(N)` option

## Property Pattern Cheat Sheet

| Pattern | Property | Code shape |
|---|---|---|
| Round-trip | `decode(encode(x)) == x` | Marshal → Unmarshal → compare |
| Idempotency | `f(f(x)) == f(x)` | Apply twice, compare to once |
| Invariant preservation | Postcondition holds after op | Assert length, ordering, membership |
| Oracle comparison | `fast(x) == slow(x)` | Run both, compare results |
| Commutativity | `f(a,b) == f(b,a)` | Swap args, compare |
| Associativity | `f(f(a,b),c) == f(a,f(b,c))` | Regroup, compare |
| Robustness | Never panics | `_, _ = Parse(input)` |
| Symmetry | `pop(push(x)) == x` | Dual ops cancel out |
| Structural | Size/ordering constraints | Assert BST/heap/size invariants |
| Stateful | Real system matches model | `rapid.Run[*stateMachine]()` |

For full code examples of each pattern, see [references/patterns.md](references/patterns.md).

## Stateful Testing Overview

Test sequences of operations against a simplified model:

```go
type myMachine struct {
    real  *MySystem          // system under test
    model map[string]string  // reference model
}

func (m *myMachine) Init(t *rapid.T) {
    m.real = New()
    m.model = make(map[string]string)
}

// Each exported method with signature `(t *rapid.T)` becomes a random operation
func (m *myMachine) Put(t *rapid.T)    { /* operate on both real + model */ }
func (m *myMachine) Get(t *rapid.T)    { /* compare real vs model */ }
func (m *myMachine) Delete(t *rapid.T) { /* operate on both */ }

// Check is called after every operation
func (m *myMachine) Check(t *rapid.T) { /* assert invariants */ }

func TestStateful(t *testing.T) {
    rapid.Check(t, rapid.Run[*myMachine]())
}
```

When failures occur, rapid shrinks both the operation sequence and arguments.

## Generator Quick Reference

```go
// Primitives
rapid.Int()                          rapid.IntRange(lo, hi)
rapid.Float64()                      rapid.Float64Range(lo, hi)
rapid.Bool()                         rapid.Byte()
rapid.String()                       rapid.StringMatching(regex)

// Collections
rapid.SliceOf(gen)                   rapid.SliceOfN(gen, min, max)
rapid.SliceOfDistinct(gen, keyFn)    rapid.MapOf(kGen, vGen)

// Selection
rapid.SampledFrom(slice)             rapid.Just(value)
rapid.OneOf(gens...)

// Custom composite
rapid.Custom(func(t *rapid.T) T { ... })

// Filtering (use sparingly — prefer direct construction)
gen.Filter(func(v T) bool { ... })
```

For advanced generators (recursive data, constrained types, concurrency), see [references/advanced.md](references/advanced.md).

## Best Practices

1. **Start simple** — "doesn't crash", round-trip, basic invariants catch many bugs
2. **Combine properties** — multiple simple properties together fully specify behavior
3. **Label draws descriptively** — `"accountBalance"` not `"x"`
4. **Keep properties fast** — each runs 100+ times; mock I/O, avoid network
5. **Prefer construction over filtering** — `SliceOfN(gen, 1, 100)` not `SliceOf(gen).Filter(nonEmpty)`
6. **Add regression tests** — when PBT finds a bug, add an example-based test for that case
7. **Tune iterations for CI** — 100 locally, 10000+ in CI via env check
8. **Reproduce failures** — use `go test -run TestName -rapid.seed=<seed>` from failure output

## Avoid `testing/quick`

The stdlib `testing/quick` package has no shrinking, limited generator control, and no stateful testing. It is effectively frozen. Always prefer `rapid` for new code.
