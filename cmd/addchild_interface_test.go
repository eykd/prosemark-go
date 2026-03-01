// Package-level compile-time assertions that drive the NewNodeAddChildIO
// interface declaration (RED until the type is introduced in addchild.go).
//
// Currently these lines fail to compile because NewNodeAddChildIO is
// undefined. Once GREEN adds the type, all four assertions are satisfied at
// compile time with no code changes to the production implementations.
package cmd

// NewNodeAddChildIO must subsume AddChildIO so that all existing command-level
// call sites that pass AddChildIO remain valid.
var _ AddChildIO = (NewNodeAddChildIO)(nil)

// NewNodeAddChildIO must subsume newNodeIO so that the combined interface can
// replace the runtime io.(newNodeIO) type assertion inside runNewMode.
var _ newNodeIO = (NewNodeAddChildIO)(nil)

// fileAddChildIO (the production implementation) must satisfy the combined
// interface without modification — it already implements both sub-interfaces.
var _ NewNodeAddChildIO = (*fileAddChildIO)(nil)

// mockAddChildIOWithNew (the test double for --new mode) must also satisfy
// NewNodeAddChildIO, confirming that existing --new tests remain valid.
var _ NewNodeAddChildIO = (*mockAddChildIOWithNew)(nil)
