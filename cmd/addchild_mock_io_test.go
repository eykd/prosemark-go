package cmd

// Stub newNodeIO methods on mockAddChildIO so that it satisfies
// NewNodeAddChildIO. These stubs are no-ops; tests that need real behaviour
// use mockAddChildIOWithNew (which shadows these via embedding).

func (m *mockAddChildIO) WriteNodeFileAtomic(_ string, _ []byte) error {
	return nil
}

func (m *mockAddChildIO) DeleteFile(_ string) error {
	return nil
}

func (m *mockAddChildIO) OpenEditor(_, _ string) error {
	return nil
}

func (m *mockAddChildIO) ReadNodeFile(_ string) ([]byte, error) {
	return nil, nil
}
