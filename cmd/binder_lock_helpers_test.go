package cmd

import (
	"bytes"
	"testing"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []string
	}{
		{"nil input", nil, nil},
		{"empty input", []byte{}, nil},
		{"only newline", []byte("\n"), nil},
		{"single line with newline", []byte("hello\n"), []string{"hello"}},
		{"single line without newline", []byte("hello"), []string{"hello"}},
		{"two lines", []byte("a\nb\n"), []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitLines() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitLines()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMergeBinderLines(t *testing.T) {
	header := []byte("<!-- prosemark-binder:v1 -->\n")
	entry0 := "- [](node00.md)"
	entry1 := "- [](node01.md)"

	tests := []struct {
		name     string
		current  []byte
		incoming []byte
		wantHas  []string
	}{
		{
			name:     "both nil",
			current:  nil,
			incoming: nil,
			wantHas:  []string{},
		},
		{
			name:     "current nil, incoming has header",
			current:  nil,
			incoming: header,
			wantHas:  []string{"<!-- prosemark-binder:v1 -->"},
		},
		{
			name:     "current empty, incoming has entry",
			current:  header,
			incoming: append(append([]byte{}, header...), []byte(entry0+"\n")...),
			wantHas:  []string{"<!-- prosemark-binder:v1 -->", entry0},
		},
		{
			name:     "merge adds new line from incoming",
			current:  append(append([]byte{}, header...), []byte(entry0+"\n")...),
			incoming: append(append([]byte{}, header...), []byte(entry1+"\n")...),
			wantHas:  []string{entry0, entry1},
		},
		{
			name:     "duplicate lines deduplicated",
			current:  append(append([]byte{}, header...), []byte(entry0+"\n")...),
			incoming: append(append([]byte{}, header...), []byte(entry0+"\n")...),
			wantHas:  []string{entry0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeBinderLines(tt.current, tt.incoming)
			for _, want := range tt.wantHas {
				if !bytes.Contains(got, []byte(want)) {
					t.Errorf("mergeBinderLines() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}
