package sqlstring

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	tmFmtWithMS = "2006-01-02 15:04:05.999"
	escaper     = "'"
	nullStr     = "NULL"

	// PostgreSQL escapes a single quote inside a string literal by doubling it.
	singleQuoteEscaper = "''"
)

// Escape escape the val for sql
func Escape(val interface{}) string {
	return EscapeInLocation(val, time.Local)
}

// PostgreSQL string literal escaping (standard_conforming_strings=on):
// - Escape single quotes by doubling them.
// - Backslash is treated as a regular character (no MySQL-style escaping).
func escapeStringLiteral(val string) string {
	return strings.ReplaceAll(val, escaper, singleQuoteEscaper)
}

func timeToString(t time.Time, loc *time.Location) string {
	if loc != nil {
		return escaper + t.In(loc).Format(tmFmtWithMS) + escaper
	}
	return escaper + t.Format(tmFmtWithMS) + escaper
}

func arrayToString(refValue reflect.Value, loc *time.Location) string {
	var res []string
	for i := 0; i < refValue.Len(); i++ {
		res = append(res, EscapeInLocation(refValue.Index(i).Interface(), loc))
	}
	return strings.Join(res, ",")
}

func bytesToString(b []byte) string {
	// PostgreSQL: decode(hex, 'hex') returns bytea.
	return "decode(" + escaper + hex.EncodeToString(b) + escaper + "," + escaper + "hex" + escaper + ")"
}

// EscapeInLocation  escape the val  with time.Location
func EscapeInLocation(val interface{}, loc *time.Location) string {
	if val == nil {
		return nullStr
	}

	switch v := val.(type) {
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return timeToString(v, loc)
	case *time.Time:
		if v == nil {
			return nullStr
		}
		// nosemgrep
		return timeToString(*v, loc)
	case []byte:
		return bytesToString(v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%.6f", v)

	case string:
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "((") && strings.HasSuffix(trimmed, "))") {
			// Treat "((...))" inputs as a search DSL payload: only make it safe as a SQL
			// string literal (escape single quotes), keep everything else intact.
			return escaper + escapeStringLiteral(v) + escaper
		}
		return escaper + escapeStringLiteral(v) + escaper
	default:
		refValue := reflect.ValueOf(v)
		if v == nil || !refValue.IsValid() {
			return nullStr
		}

		if refValue.Kind() == reflect.Ptr && refValue.IsNil() {
			return nullStr
		}

		if refValue.Kind() == reflect.Ptr && !refValue.IsZero() {
			return EscapeInLocation(reflect.Indirect(refValue).Interface(), loc)
		}

		if refValue.Kind() == reflect.Array || refValue.Kind() == reflect.Slice {
			//slice or array
			if refValue.Len() == 0 {
				return nullStr
			}
			return arrayToString(refValue, loc)
		}

		stringifyData, err := json.Marshal(v)
		if err != nil {
			return nullStr
		}
		return escaper + escapeStringLiteral(string(stringifyData)) + escaper

	}
}

// Format format the sql with args
func Format(query string, args ...interface{}) string {

	if len(args) == 0 {
		return query
	}

	var sql strings.Builder
	replaceIndex := 0
	for _, v := range query {
		if v == '?' {
			if len(args) > replaceIndex {
				sql.WriteString(Escape(args[replaceIndex]))
				replaceIndex++
				continue
			}
		}
		sql.WriteRune(v)
	}
	return sql.String()
}

// FormatInLocation format the sql with args
func FormatInLocation(query string, loc *time.Location, args ...interface{}) string {

	if len(args) == 0 {
		return query
	}

	var sql strings.Builder
	replaceIndex := 0
	for _, v := range query {
		if v == '?' {
			if len(args) > replaceIndex {
				sql.WriteString(EscapeInLocation(args[replaceIndex], loc))
				replaceIndex++
				continue
			}
		}
		sql.WriteRune(v)
	}
	return sql.String()
}

// SetSingleQuoteEscaper set the singleQuoteEscaper
// default: doubled single quote (PostgreSQL standard string literal escaping)
func SetSingleQuoteEscaper(escaper string) {
	singleQuoteEscaper = escaper
}
