# Advanced Techniques

## Custom Composite Generators

Use `rapid.Custom` for structs. Shrinking works automatically — each `Draw` shrinks independently.

```go
type User struct {
    Name  string
    Age   int
    Email string
}

func genUser() *rapid.Generator[User] {
    return rapid.Custom(func(t *rapid.T) User {
        return User{
            Name:  rapid.StringMatching(`[A-Z][a-z]{2,15}`).Draw(t, "name"),
            Age:   rapid.IntRange(0, 150).Draw(t, "age"),
            Email: rapid.StringMatching(`[a-z]+@[a-z]+\.[a-z]{2,3}`).Draw(t, "email"),
        }
    })
}
```

## Constrained Data Generation

**Prefer direct construction over filtering:**

```go
// BAD — slow if most values rejected
rapid.SliceOf(rapid.Int()).Filter(func(s []int) bool { return len(s) > 0 })

// GOOD — generates exactly what you need
rapid.SliceOfN(rapid.Int(), 1, 100)
```

**Complex constraints with `rapid.Custom`:**

```go
// Sorted, non-empty slice of distinct positive ints
func genSortedDistinctPositive() *rapid.Generator[[]int] {
    return rapid.Custom(func(t *rapid.T) []int {
        items := rapid.SliceOfN(rapid.IntRange(1, 10000), 1, 50).Draw(t, "items")

        seen := make(map[int]bool)
        unique := items[:0]
        for _, v := range items {
            if !seen[v] {
                seen[v] = true
                unique = append(unique, v)
            }
        }
        sort.Ints(unique)
        return unique
    })
}
```

## Recursive / Tree-Shaped Data

Use depth parameter to prevent infinite recursion:

```go
type Tree struct {
    Value    int
    Children []*Tree
}

func genTree(maxDepth int) *rapid.Generator[*Tree] {
    if maxDepth <= 0 {
        return rapid.Custom(func(t *rapid.T) *Tree {
            return &Tree{Value: rapid.Int().Draw(t, "leaf")}
        })
    }
    return rapid.Custom(func(t *rapid.T) *Tree {
        value := rapid.Int().Draw(t, "value")
        numChildren := rapid.IntRange(0, 3).Draw(t, "numChildren")

        children := make([]*Tree, numChildren)
        for i := range children {
            children[i] = genTree(maxDepth - 1).Draw(t, fmt.Sprintf("child_%d", i))
        }

        return &Tree{Value: value, Children: children}
    })
}
```

## Concurrent Code Testing

Generate operations, run concurrently, verify final state:

```go
func TestConcurrentMapSafety(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        m := NewConcurrentMap[string, int]()
        numGoroutines := rapid.IntRange(2, 10).Draw(t, "goroutines")
        opsPerGoroutine := rapid.IntRange(10, 50).Draw(t, "ops")

        var wg sync.WaitGroup
        for g := 0; g < numGoroutines; g++ {
            wg.Add(1)
            go func(id int) {
                defer wg.Done()
                for i := 0; i < opsPerGoroutine; i++ {
                    key := fmt.Sprintf("key-%d-%d", id, i)
                    m.Set(key, i)
                    _, _ = m.Get(key)
                }
            }(g)
        }
        wg.Wait()

        if m.Len() < 0 {
            t.Fatal("negative length")
        }
    })
}
```

Always combine with race detector: `go test -race`

## Seeding and Reproducibility

When a test fails, rapid prints the seed:

```
--- FAIL: TestMyProperty (0.03s)
    [rapid] seed: 14832957610483726591
```

Reproduce with: `go test -run TestMyProperty -rapid.seed=14832957610483726591`

Same seed → same generated values → same shrink path.

## CI Iteration Tuning

```go
func TestCriticalProperty(t *testing.T) {
    iterCount := 100
    if os.Getenv("CI") != "" {
        iterCount = 10000
    }
    rapid.Check(t, func(t *rapid.T) {
        // ...
    }, rapid.Checks(iterCount))
}
```

## PBT vs. Go Fuzzing (`go test -fuzz`)

| Aspect | Property-Based Testing | Go Fuzzing |
|---|---|---|
| Input generation | Random with shrinking | Coverage-guided mutation |
| Duration | Fixed iterations (fast) | Runs until stopped |
| Shrinking | Automatic, principled | Basic minimization |
| Complex inputs | Rich generator combinators | Byte-level mutation |
| Typical use | Development-time testing | Long-running CI jobs |

They complement each other. Use PBT for fast development feedback with rich generators; fuzzing for deep, long-running exploration of byte-level edge cases.

Bridge them:

```go
func FuzzParser(f *testing.F) {
    f.Add([]byte("valid input"))
    f.Add([]byte(""))
    f.Add([]byte("{"))

    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = Parse(data) // never panic
    })
}
```

## gopter (Alternative Library)

`github.com/leanovate/gopter` — ScalaCheck-inspired, more verbose API. Key differences from rapid:

- Shrinking requires separate shrinker per generator (not automatic)
- Combinator-based (functional) rather than draw-based (imperative)
- Less actively maintained
- More verbose API

Prefer `rapid` for new projects. Only relevant if maintaining existing `gopter` codebases.
