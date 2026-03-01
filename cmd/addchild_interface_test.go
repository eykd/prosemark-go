// Package-level compile-time assertions that guard the NewNodeAddChildIO
// interface composition.
package cmd

// NewNodeAddChildIO must subsume AddChildIO so that all existing command-level
// call sites that pass AddChildIO remain valid.
var _ AddChildIO = (NewNodeAddChildIO)(nil)

// NewNodeAddChildIO must subsume newNodeIO so that the combined interface
// expresses --new mode capability in the type system.
var _ newNodeIO = (NewNodeAddChildIO)(nil)

// fileAddChildIO (the production implementation) must satisfy the combined
// interface without modification — it already implements both sub-interfaces.
var _ NewNodeAddChildIO = (*fileAddChildIO)(nil)

// mockAddChildIOWithNew (the test double for --new mode) must also satisfy
// NewNodeAddChildIO, confirming that existing --new tests remain valid.
var _ NewNodeAddChildIO = (*mockAddChildIOWithNew)(nil)
