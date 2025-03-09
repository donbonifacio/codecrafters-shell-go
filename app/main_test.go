package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestEcho(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic echo",
			input:    "echo hello",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "echo",
			expected: "",
		},
		{
			name:     "special characters",
			input:    "echo !@#$%^&*()",
			expected: "!@#$%^&*()",
		},
		{
			name:     "two strings",
			input:    "echo a b",
			expected: "a b",
		},
		{
			name:     "two strings lots of spaces",
			input:    "echo a        b",
			expected: "a b",
		},
		{
			name:     "quotes",
			input:    "echo 'a b'",
			expected: "a b",
		},
		{
			name:     "quotes joint with quotes",
			input:    "echo 'a b''c d'",
			expected: "a bc d",
		},
		{
			name:     "quotes joint with quotes but has space",
			input:    "echo 'a b' 'c d'",
			expected: "a b c d",
		},
		{
			name:     "double quotes",
			input:    "echo \"a b\"",
			expected: "a b",
		},
		{
			name:     "double quotes with inner quote",
			input:    "echo \"bar\"  \"shell's\"  \"foo\"",
			expected: "bar shell's foo",
		},
	}

	for _, testData := range tests {
		t.Run(testData.name, func(t *testing.T) {
			// Create a pipe to capture stdout
			oldStdout := os.Stdout
			defer func() { os.Stdout = oldStdout }()
			r, w, _ := os.Pipe()
			os.Stdout = w
			input := CommandArgs{Raw: testData.input, Parts: processParts(testData.input)}
			err := echo(&input)
			// Read output
			w.Close()
			output, _ := io.ReadAll(r)
			result := strings.TrimRight(string(output), "\n")
			if err != nil {
				t.Errorf("Echo(%s) = %s; want %s", testData.input, result, testData.expected)
			}
			if result != testData.expected {
				t.Errorf("Echo(%s) = \"%s\"; want \"%s\"\nParts: %v", testData.input, result, testData.expected, input.Parts)
			}
		})
	}
}
