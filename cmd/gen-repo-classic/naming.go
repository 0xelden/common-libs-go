package main

import (
	"strings"
	"unicode"
)

var knownTablePrefixes = []string{
	"md_",
	"usm_",
}

var preservePluralModuleNames = map[string]struct{}{
	"sim_recplan_notes": {},
}

func pascalCase(input string) string {
	input = strings.ReplaceAll(input, ".", "_")
	input = strings.ReplaceAll(input, "-", "_")
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '_' || r == ' ' || r == '.'
	})
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		builder.WriteRune(unicode.ToUpper(runes[0]))
		for _, r := range runes[1:] {
			builder.WriteRune(unicode.ToLower(r))
		}
	}
	return builder.String()
}

func snakeCase(input string) string {
	if input == "" {
		return input
	}
	input = strings.TrimSpace(input)
	var builder strings.Builder
	var prevUnderscore bool
	for i, r := range input {
		switch {
		case r == '-' || r == ' ' || r == '.':
			if !prevUnderscore && builder.Len() > 0 {
				builder.WriteRune('_')
				prevUnderscore = true
			}
		case unicode.IsUpper(r):
			if i > 0 && !prevUnderscore {
				builder.WriteRune('_')
			}
			builder.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		default:
			builder.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		}
	}
	return strings.Trim(builder.String(), "_")
}

func singularizeSnake(input string) string {
	if input == "" {
		return input
	}
	parts := strings.Split(input, "_")
	if len(parts) == 0 {
		return input
	}
	last := parts[len(parts)-1]
	if last == "" {
		return input
	}
	lower := strings.ToLower(last)
	irregulars := map[string]string{
		"menus":  "menu",
		"status": "status",
	}
	if singular, ok := irregulars[lower]; ok {
		parts[len(parts)-1] = singular
		return strings.Join(parts, "_")
	}
	switch {
	case strings.HasSuffix(lower, "ies") && len(lower) > 3:
		last = last[:len(last)-3] + "y"
	case strings.HasSuffix(lower, "s") && len(lower) > 1 &&
		!strings.HasSuffix(lower, "ss") &&
		!strings.HasSuffix(lower, "is"):
		last = last[:len(last)-1]
	}
	parts[len(parts)-1] = last
	return strings.Join(parts, "_")
}

func moduleNameForTable(tableName string) string {
	base := snakeCase(tableName)
	for _, prefix := range knownTablePrefixes {
		if strings.HasPrefix(base, prefix) {
			trimmed := strings.TrimPrefix(base, prefix)
			if trimmed != "" {
				base = trimmed
			}
			break
		}
	}

	if _, ok := preservePluralModuleNames[base]; ok {
		return base
	}

	base = singularizeSnake(base)
	if base == "" {
		return snakeCase(tableName)
	}
	return base
}

func structNameForTable(tableName string) string {
	return pascalCase(moduleNameForTable(tableName))
}
