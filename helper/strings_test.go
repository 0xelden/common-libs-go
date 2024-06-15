package helper

import (
	"testing"
)

func TestIsValidQuotes(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Hello, 'world'", true},                         // Matched single quotes
		{`It's a \"test\"`, false},                       // Unmatched quotes
		{`It\'s a "test"`, true},                         // Matched single and double quotes
		{"Hello, 'world", false},                         // Unmatched single quote
		{"She said, 'It\\'s a \"test\"'", true},          // Mixed matched quotes with escapes
		{"\"Unmatched quote", false},                     // Unmatched double quote
		{"No quotes here", true},                         // No quotes at all
		{"No.quotes.here", true},                         // No quotes at all
		{"123456", true},                                 // No quotes at all
		{"\"Escaped \\\" inside\" and 'another'", true},  // Matched double quotes with escaped double quote
		{"'Escaped \\' inside' and \"mismatched", false}, // Mismatched double quote after single quote section
		{"'Single' and \"double\"", true},                // Separate matched single and double quotes
		{`'admin\'@foo.bar'`, true},                      // Separate matched single and double quotes
	}
	for _, test := range tests {
		result := IsValidQuotes(test.input)
		if result != test.expected {
			t.Errorf("Failed. Input: %-35s | Expected: %v | Got: %v\n", test.input, test.expected, result)
		}
	}
}

func TestEscapeSingleQuoteSql(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`It\'s a test`, `It''s a test`},
		{`It's a test`, `It's a test`},
		{`She said, \'Hello\'`, `She said, ''Hello''`},
		{`Path C:\\Program Files\\`, `Path C:\\Program Files\\`},     // No change expected
		{`Escape test with no quotes`, `Escape test with no quotes`}, // No quotes to escape
		{`'O\'Relly'`, `'O''Relly'`},
		{`\'First\' and \'Second\'`, `''First'' and ''Second''`},
	}
	for _, test := range tests {
		result := EscapeSingleQuoteSql(test.input)
		t.Logf("Input: %-30s | Expected: %-30s | Got: %-30s\n", test.input, test.expected, result)
		if result != test.expected {
			t.Errorf("Failed. Input: %-30s | Expected: %-30s | Got: %-30s\n", test.input, test.expected, result)
		}
	}
}
func TestNormalizeUpper(t *testing.T) {
	defaultStr := "DEFAULT"

	tests := []struct {
		name       string
		input      any
		defaultStr string
		expected   string
	}{
		{"String input", "hello", defaultStr, "HELLO"},
		{"String with spaces", "  hello  ", defaultStr, "HELLO"},
		{"Pointer to string", Ptr("world"), defaultStr, "WORLD"},
		{"Pointer to empty string", Ptr(""), defaultStr, "DEFAULT"},
		{"Nil input", nil, defaultStr, "DEFAULT"},
		{"Empty string", "", defaultStr, "DEFAULT"},
		{"Integer input", 123, defaultStr, "123"},
		{"Boolean input", true, defaultStr, "TRUE"},
		{"Non-string type (float)", 3.14, defaultStr, "3.14"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeUpper(test.input, test.defaultStr)
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestNormalizeLower(t *testing.T) {
	defaultStr := "default"

	tests := []struct {
		name       string
		input      any
		defaultStr string
		expected   string
	}{
		{"String input", "HELLO", defaultStr, "hello"},
		{"String with spaces", "  HELLO  ", defaultStr, "hello"},
		{"Pointer to string", Ptr("WORLD"), defaultStr, "world"},
		{"Pointer to empty string", Ptr(""), defaultStr, "default"},
		{"Nil input", nil, defaultStr, "default"},
		{"Empty string", "", defaultStr, "default"},
		{"Integer input", 123, defaultStr, "123"},
		{"Boolean input", false, defaultStr, "false"},
		{"Non-string type (float)", 3.14, defaultStr, "3.14"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeLower(test.input, test.defaultStr)
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestHashString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HelloWorld", "F8F07AF"},     // Expected output from PostgreSQL
		{"  HelloWorld  ", "F8F07AF"}, // Spaces should not affect output
		{"helloworld", "F8F07AF"},     // Case-insensitive
		{"HELLOworld", "F8F07AF"},     // Case-insensitive
		{"Different", "C690D11"},      // Different input, different hash
		//{"A107104Plate - SS400 7,8 x 834 x 6000Plate - SS400 7,8 x 834 x 6000", "01AD263"},
		//{"J022001Besi Beton 22 x 6000Besi Beton 22 x 6000", "01AD263"},
		{"A107104Plate - SS400 7,8 x 834 x 6000Plate - SS400 7,8 x 834 x 6000", "E4933BE"},
		{"J022001Besi Beton 22 x 6000Besi Beton 22 x 6000", "4AD20AA"},
	}
	for _, tt := range tests {
		got := HashString(tt.input)
		if got != tt.expected {
			t.Errorf("HashString(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
