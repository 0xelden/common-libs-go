package main

import (
	"strings"
	"unicode"

	"github.com/go-errors/errors"
)

func ParseCreateTable(payload string) (*Table, error) {
	statements := splitStatements(payload)
	for _, statement := range statements {
		table, err := parseCreateStatement(statement)
		if err == nil {
			return table, nil
		}
	}
	return nil, errors.Errorf("no CREATE TABLE statement found")
}

func parseCreateStatement(stmt string) (*Table, error) {
	trimmed := strings.TrimSpace(stmt)
	if trimmed == "" {
		return nil, errors.Errorf("empty statement")
	}

	upper := strings.ToUpper(trimmed)
	idx := strings.Index(upper, "CREATE TABLE")
	if idx == -1 {
		return nil, errors.Errorf("not a CREATE TABLE statement")
	}

	after := strings.TrimSpace(trimmed[idx+len("CREATE TABLE"):])
	if strings.HasPrefix(strings.ToUpper(after), "IF NOT EXISTS") {
		after = strings.TrimSpace(after[len("IF NOT EXISTS"):])
	}

	openIdx := strings.Index(after, "(")
	if openIdx == -1 {
		return nil, errors.Errorf("missing column definitions")
	}

	tableIdent := strings.TrimSpace(after[:openIdx])
	schema, name := splitSchemaAndTable(tableIdent)
	if name == "" {
		return nil, errors.Errorf("table name missing")
	}

	columnSection, err := extractColumnSection(after[openIdx:])
	if err != nil {
		return nil, err
	}

	columns, pkCols, err := parseColumns(columnSection)
	if err != nil {
		return nil, err
	}

	if len(columns) == 0 {
		return nil, errors.Errorf("no columns parsed")
	}

	for i := range columns {
		key := strings.ToLower(columns[i].Name)
		if _, ok := pkCols[key]; ok {
			columns[i].IsPrimary = true
			columns[i].NotNull = true
		}
	}

	return &Table{
		Schema:  schema,
		Name:    name,
		Columns: columns,
	}, nil
}

func splitSchemaAndTable(identifier string) (schema, table string) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", ""
	}
	parts := strings.Split(identifier, ".")
	if len(parts) == 1 {
		return "", trimIdentifier(parts[0])
	}
	return trimIdentifier(parts[0]), trimIdentifier(strings.Join(parts[1:], "."))
}

func trimIdentifier(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.Trim(value, "`")
	return value
}

func extractColumnSection(payload string) (string, error) {
	payload = strings.TrimSpace(payload)
	if !strings.HasPrefix(payload, "(") {
		return "", errors.Errorf("column definition missing opening parenthesis")
	}

	depth := 0
	start := -1
	inSingle := false
	inDouble := false

	for i := 0; i < len(payload); i++ {
		ch := payload[i]
		switch ch {
		case '\'':
			if inDouble {
				break
			}
			if inSingle && i+1 < len(payload) && payload[i+1] == '\'' {
				i++
			} else {
				inSingle = !inSingle
			}
		case '"':
			if inSingle {
				break
			}
			inDouble = !inDouble
		case '(':
			if inSingle || inDouble {
				break
			}
			depth++
			if depth == 1 {
				start = i + 1
			}
		case ')':
			if inSingle || inDouble {
				break
			}
			depth--
			if depth == 0 {
				if start == -1 {
					return "", errors.Errorf("column section start not found")
				}
				return payload[start:i], nil
			}
		}
	}
	return "", errors.Errorf("unterminated column definition")
}

func parseColumns(section string) ([]Column, map[string]struct{}, error) {
	fragments := splitComma(section)
	columns := make([]Column, 0, len(fragments))
	pkColumns := map[string]struct{}{}

	for _, fragment := range fragments {
		if fragment == "" {
			continue
		}

		upper := strings.ToUpper(fragment)
		switch {
		case strings.HasPrefix(upper, "CONSTRAINT") && strings.Contains(upper, "PRIMARY KEY"):
			for _, name := range extractPkColumns(fragment) {
				pkColumns[strings.ToLower(name)] = struct{}{}
			}
			continue
		case strings.HasPrefix(upper, "PRIMARY KEY"):
			for _, name := range extractPkColumns(fragment) {
				pkColumns[strings.ToLower(name)] = struct{}{}
			}
			continue
		}

		column, err := parseColumnDefinition(fragment)
		if err != nil {
			return nil, nil, err
		}
		if column.IsPrimary {
			pkColumns[strings.ToLower(column.Name)] = struct{}{}
		}
		columns = append(columns, column)
	}
	return columns, pkColumns, nil
}

func parseColumnDefinition(definition string) (Column, error) {
	token, rest := nextToken(definition)
	if token == "" || rest == "" {
		return Column{}, errors.Errorf("invalid column definition: %s", definition)
	}
	name := trimIdentifier(token)
	typeExpr, attrs := splitTypeAndAttrs(rest)
	if typeExpr == "" {
		return Column{}, errors.Errorf("column %s missing type", name)
	}

	column := Column{
		Name:                 name,
		RawType:              normalizeWhitespace(typeExpr),
		NotNull:              containsNotNull(attrs),
		IsPrimary:            containsPrimary(attrs),
		HasDefault:           containsKeyword(attrs, "DEFAULT"),
		IsGenerated:          containsGenerated(attrs),
		GenerationExpression: extractGeneratedExpression(definition),
	}
	if column.IsPrimary {
		column.NotNull = true
	}
	return column, nil
}

func extractPkColumns(definition string) []string {
	start := strings.Index(definition, "(")
	end := strings.LastIndex(definition, ")")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	inner := definition[start+1 : end]
	parts := splitComma(inner)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = trimIdentifier(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitComma(section string) []string {
	var result []string
	depth := 0
	inSingle := false
	inDouble := false
	start := 0

	for i := 0; i < len(section); i++ {
		ch := section[i]
		switch ch {
		case '\'':
			if inDouble {
				break
			}
			if inSingle && i+1 < len(section) && section[i+1] == '\'' {
				i++
			} else {
				inSingle = !inSingle
			}
		case '"':
			if inSingle {
				break
			}
			inDouble = !inDouble
		case '(':
			if !inSingle && !inDouble {
				depth++
			}
		case ')':
			if !inSingle && !inDouble && depth > 0 {
				depth--
			}
		case ',':
			if !inSingle && !inDouble && depth == 0 {
				segment := strings.TrimSpace(section[start:i])
				result = append(result, segment)
				start = i + 1
			}
		}
	}

	last := strings.TrimSpace(section[start:])
	if last != "" {
		result = append(result, last)
	}
	return result
}

func nextToken(input string) (string, string) {
	input = strings.TrimSpace(input)
	for i, r := range input {
		if unicode.IsSpace(r) {
			return input[:i], input[i+1:]
		}
	}
	return input, ""
}

func splitTypeAndAttrs(section string) (string, []string) {
	tokens := strings.Fields(section)
	if len(tokens) == 0 {
		return "", nil
	}

	idx := len(tokens)
	for i := 0; i < len(tokens); i++ {
		token := strings.ToUpper(tokens[i])
		if isConstraintToken(token, tokens, i) {
			idx = i
			break
		}
	}

	return strings.Join(tokens[:idx], " "), tokens[idx:]
}

func isConstraintToken(token string, tokens []string, idx int) bool {
	switch token {
	case "CONSTRAINT", "DEFAULT", "CHECK", "UNIQUE", "REFERENCES", "NULL", "GENERATED":
		return true
	case "NOT":
		return idx+1 < len(tokens) && strings.ToUpper(tokens[idx+1]) == "NULL"
	case "PRIMARY", "FOREIGN":
		return true
	}
	return false
}

func containsNotNull(tokens []string) bool {
	for i := 0; i < len(tokens); i++ {
		if strings.ToUpper(tokens[i]) == "NOT" && i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "NULL" {
			return true
		}
	}
	return false
}

func containsPrimary(tokens []string) bool {
	for i := 0; i < len(tokens); i++ {
		if strings.ToUpper(tokens[i]) == "PRIMARY" && i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "KEY" {
			return true
		}
	}
	return false
}

func containsKeyword(tokens []string, keyword string) bool {
	keyword = strings.ToUpper(keyword)
	for _, token := range tokens {
		if strings.ToUpper(token) == keyword {
			return true
		}
	}
	return false
}

func containsGenerated(tokens []string) bool {
	return containsKeyword(tokens, "GENERATED") && containsKeyword(tokens, "AS")
}

func extractGeneratedExpression(definition string) string {
	upper := strings.ToUpper(definition)
	idx := strings.Index(upper, "GENERATED ")
	if idx == -1 {
		return ""
	}

	segment := definition[idx:]
	asIdx := strings.Index(strings.ToUpper(segment), " AS ")
	if asIdx == -1 {
		return ""
	}

	afterAs := segment[asIdx+4:]
	start := strings.Index(afterAs, "(")
	if start == -1 {
		return ""
	}

	depth := 0
	inSingle := false
	inDouble := false
	for i := start; i < len(afterAs); i++ {
		ch := afterAs[i]
		switch ch {
		case '\'':
			if inDouble {
				continue
			}
			if inSingle && i+1 < len(afterAs) && afterAs[i+1] == '\'' {
				i++
				continue
			}
			inSingle = !inSingle
		case '"':
			if inSingle {
				continue
			}
			inDouble = !inDouble
		case '(':
			if !inSingle && !inDouble {
				depth++
			}
		case ')':
			if !inSingle && !inDouble {
				depth--
				if depth == 0 {
					return strings.TrimSpace(afterAs[start+1 : i])
				}
			}
		}
	}
	return ""
}

func splitStatements(payload string) []string {
	var statements []string
	var builder strings.Builder
	depth := 0
	inSingle := false
	inDouble := false

	for i := 0; i < len(payload); i++ {
		ch := payload[i]

		switch ch {
		case '\'':
			if inDouble {
				break
			}
			if inSingle && i+1 < len(payload) && payload[i+1] == '\'' {
				builder.WriteByte(ch)
				i++
				ch = payload[i]
			} else {
				inSingle = !inSingle
			}
		case '"':
			if inSingle {
				break
			}
			inDouble = !inDouble
		case '(':
			if !inSingle && !inDouble {
				depth++
			}
		case ')':
			if !inSingle && !inDouble && depth > 0 {
				depth--
			}
		case ';':
			if !inSingle && !inDouble && depth == 0 {
				statements = append(statements, builder.String())
				builder.Reset()
				continue
			}
		}

		builder.WriteByte(ch)
	}

	if builder.Len() > 0 {
		statements = append(statements, builder.String())
	}
	return statements
}

func normalizeWhitespace(input string) string {
	input = strings.TrimSpace(input)
	fields := strings.Fields(strings.ToLower(input))
	return strings.Join(fields, " ")
}
