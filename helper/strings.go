package helper

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// IsValidQuotes checks if all single and double quotes in the string are properly matched.
func IsValidQuotes(text string) bool {
	inSingleQuote := false // Tracks if we're inside a single-quoted section
	inDoubleQuote := false // Tracks if we're inside a double-quoted section

	for i := 0; i < len(text); i++ {
		char := text[i]

		// Check for escaped single or double quote (e.g., \' or \")
		if char == '\\' && i+1 < len(text) {
			if text[i+1] == '\'' || text[i+1] == '"' {
				i++ // Skip the escaped quote
				continue
			}
		}

		// Toggle inSingleQuote if we encounter an unescaped single quote and not inside a double-quote section
		if char == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
		} else if char == '"' && !inSingleQuote { // Toggle inDoubleQuote if we encounter an unescaped double quote and not inside a single-quote section
			inDoubleQuote = !inDoubleQuote
		}
	}

	// Both inSingleQuote and inDoubleQuote should be false if all quotes are matched
	return !inSingleQuote && !inDoubleQuote
}

// EscapeSingleQuoteSql converts \' to ” for SQL compatibility
func EscapeSingleQuoteSql(text string) string {
	replacer := strings.NewReplacer(`\'`, `''`)
	return replacer.Replace(text)
}

func NormalizeUpper(data any, defaultStr ...string) string {
	return strings.ToUpper(strings.TrimSpace(Coalesce(append([]string{ToString(data)}, defaultStr...)...)))
}

func NormalizeLower(data any, defaultStr ...string) string {
	return strings.ToLower(strings.TrimSpace(Coalesce(append([]string{ToString(data)}, defaultStr...)...)))
}

// HashString trims spaces, converts to lowercase, hashes with SHA-256,
// takes the last 7 chars of the hex digest, and converts to uppercase.
func HashString(input string) string {
	trimmed := strings.ToLower(strings.TrimSpace(input)) // Trim and lowercase
	hash := sha256.Sum256([]byte(trimmed))               // Compute SHA-256
	hashHex := hex.EncodeToString(hash[:])               // Convert to hex
	return strings.ToUpper(hashHex[len(hashHex)-7:])     // Take last 7 chars, uppercase
}
