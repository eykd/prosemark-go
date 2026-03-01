package cmd

import "testing"

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal ASCII path unchanged",
			input: "/home/user/my-project",
			want:  "/home/user/my-project",
		},
		{
			name:  "ANSI escape sequence ESC replaced leaving rest intact",
			input: "/path/\x1b[2J/evil",
			want:  "/path/?[2J/evil",
		},
		{
			name:  "null byte replaced",
			input: "/path/\x00null",
			want:  "/path/?null",
		},
		{
			name:  "newline replaced",
			input: "/path/with\nnewline",
			want:  "/path/with?newline",
		},
		{
			name:  "carriage return replaced",
			input: "/path/with\rCR",
			want:  "/path/with?CR",
		},
		{
			name:  "tab replaced",
			input: "/path/with\ttab",
			want:  "/path/with?tab",
		},
		{
			name:  "DEL byte replaced",
			input: "/path/with\x7fDEL",
			want:  "/path/with?DEL",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "printable ASCII-only path including tilde unchanged",
			input: "/home/user/~/.config",
			want:  "/home/user/~/.config",
		},
		{
			name:  "multiple control bytes all replaced",
			input: "\x01\x1b[2J\x00",
			want:  "??[2J?",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePath(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
