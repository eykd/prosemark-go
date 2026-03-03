# Property Patterns — Full Code Examples

## Round-Trip (Encode/Decode)

```go
func TestJSONRoundTrip(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        original := genUser().Draw(t, "user")

        encoded, err := json.Marshal(original)
        if err != nil {
            t.Fatal(err)
        }

        var decoded User
        if err := json.Unmarshal(encoded, &decoded); err != nil {
            t.Fatal(err)
        }

        if original != decoded {
            t.Fatalf("round-trip failed:\n  original: %+v\n  decoded:  %+v", original, decoded)
        }
    })
}
```

Applies to: JSON, protobuf, DB serialization, URL encoding, compression, encryption/decryption.

## Idempotency

```go
func TestSortIdempotent(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        items := rapid.SliceOf(rapid.Int()).Draw(t, "items")

        once := sortCopy(items)
        twice := sortCopy(once)

        if !slices.Equal(once, twice) {
            t.Fatalf("sorting is not idempotent")
        }
    })
}
```

Other idempotent ops: `strings.TrimSpace`, `filepath.Clean`, `http.CanonicalHeaderKey`, deduplication, normalization.

## Invariant Preservation (Sort Example)

Three properties together fully specify sort:

```go
// Property 1: Output is ordered
func TestSortOutputIsOrdered(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        items := rapid.SliceOf(rapid.Int()).Draw(t, "items")
        sorted := sortCopy(items)

        for i := 1; i < len(sorted); i++ {
            if sorted[i] < sorted[i-1] {
                t.Fatalf("not sorted at index %d: %d > %d", i, sorted[i-1], sorted[i])
            }
        }
    })
}

// Property 2: Length preserved
func TestSortPreservesLength(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        items := rapid.SliceOf(rapid.Int()).Draw(t, "items")
        sorted := sortCopy(items)

        if len(sorted) != len(items) {
            t.Fatalf("sort changed length: %d -> %d", len(items), len(sorted))
        }
    })
}

// Property 3: Elements preserved
func TestSortPreservesElements(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        items := rapid.SliceOf(rapid.Int()).Draw(t, "items")
        sorted := sortCopy(items)

        origCounts := countElements(items)
        sortCounts := countElements(sorted)

        if !maps.Equal(origCounts, sortCounts) {
            t.Fatalf("sort changed elements")
        }
    })
}
```

## Oracle / Model Comparison

```go
func TestCustomMapMatchesStdlib(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        key := rapid.String().Draw(t, "key")
        value := rapid.Int().Draw(t, "value")

        stdMap := make(map[string]int)
        stdMap[key] = value

        custom := NewCustomMap[string, int]()
        custom.Set(key, value)

        got, ok := custom.Get(key)
        if !ok || got != value {
            t.Fatalf("custom map disagrees: Get(%q) = (%d, %v), want (%d, true)",
                key, got, ok, value)
        }
    })
}
```

Also useful with `quick.CheckEqual(reference, optimized, nil)` for simple cases.

## Commutativity & Associativity

```go
func TestMergeCommutative(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        a := genConfig().Draw(t, "a")
        b := genConfig().Draw(t, "b")

        if !Equal(Merge(a, b), Merge(b, a)) {
            t.Fatal("merge is not commutative")
        }
    })
}

func TestMergeAssociative(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        a := genConfig().Draw(t, "a")
        b := genConfig().Draw(t, "b")
        c := genConfig().Draw(t, "c")

        left := Merge(Merge(a, b), c)
        right := Merge(a, Merge(b, c))

        if !Equal(left, right) {
            t.Fatal("merge is not associative")
        }
    })
}
```

## Robustness / No Crash

```go
func TestParserNeverPanics(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        input := rapid.SliceOf(rapid.Byte()).Draw(t, "input")
        _, _ = Parse(input) // just don't panic
    })
}
```

Essentially fuzzing. Finds nil dereferences, index-out-of-bounds, infinite loops.

## Symmetry / Dual Operations

```go
func TestPushPopSymmetry(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        s := NewStack[int]()
        value := rapid.Int().Draw(t, "value")

        s.Push(value)
        popped, err := s.Pop()

        if err != nil {
            t.Fatal(err)
        }
        if popped != value {
            t.Fatalf("Push(%d) then Pop() = %d", value, popped)
        }
    })
}
```

## Structural Properties

```go
func TestBSTSizeAfterInsert(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        tree := NewBST[int]()
        items := rapid.SliceOfN(rapid.Int(), 1, 100).Draw(t, "items")

        for _, item := range items {
            tree.Insert(item)
        }

        if tree.Size() > len(items) {
            t.Fatalf("tree has more elements (%d) than inserted (%d)", tree.Size(), len(items))
        }
        if tree.Size() < 1 {
            t.Fatal("tree should have at least one element")
        }
    })
}
```

## Stateful Testing (Key-Value Store)

```go
type kvStateMachine struct {
    store *KVStore
    model map[string]string
}

func (sm *kvStateMachine) Init(t *rapid.T) {
    sm.store = NewKVStore()
    sm.model = make(map[string]string)
}

func (sm *kvStateMachine) Put(t *rapid.T) {
    key := rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "key")
    value := rapid.String().Draw(t, "value")
    sm.store.Put(key, value)
    sm.model[key] = value
}

func (sm *kvStateMachine) Get(t *rapid.T) {
    key := rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "key")
    got, gotOk := sm.store.Get(key)
    expected, expectedOk := sm.model[key]

    if gotOk != expectedOk {
        t.Fatalf("Get(%q): existence mismatch: got %v, want %v", key, gotOk, expectedOk)
    }
    if got != expected {
        t.Fatalf("Get(%q): value mismatch: got %q, want %q", key, got, expected)
    }
}

func (sm *kvStateMachine) Delete(t *rapid.T) {
    key := rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "key")
    sm.store.Delete(key)
    delete(sm.model, key)
}

func (sm *kvStateMachine) Check(t *rapid.T) {
    if sm.store.Len() != len(sm.model) {
        t.Fatalf("length mismatch: store=%d, model=%d", sm.store.Len(), len(sm.model))
    }
}

func TestKVStoreStateful(t *testing.T) {
    rapid.Check(t, rapid.Run[*kvStateMachine]())
}
```

Shrunk output shows minimal operation sequence to reproduce the bug.

## Real-World Example: URL Shortener

Four properties together provide thorough specification:

```go
func genURL() *rapid.Generator[string] {
    return rapid.Custom(func(t *rapid.T) string {
        scheme := rapid.SampledFrom([]string{"http", "https"}).Draw(t, "scheme")
        host := rapid.StringMatching(`[a-z]{3,10}\.(com|org|net)`).Draw(t, "host")
        path := rapid.StringMatching(`(/[a-z0-9]{1,8}){0,3}`).Draw(t, "path")
        return scheme + "://" + host + path
    })
}

// 1. Round-trip: shorten then resolve gives original
func TestShortenResolveRoundTrip(t *testing.T) { /* ... */ }

// 2. Determinism: same URL → same short code
func TestShortenDeterministic(t *testing.T) { /* ... */ }

// 3. Uniqueness: different URLs → different short codes
func TestShortenUnique(t *testing.T) { /* ... */ }

// 4. Format validity: short codes are valid URL paths
func TestShortCodeFormat(t *testing.T) { /* ... */ }
```
