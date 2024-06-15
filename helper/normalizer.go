package helper

import (
	"reflect"
	"strings"
)

func NormalizeMobileNumber(mobile string, countryCode *string) string {
	cc := "+62"
	if countryCode != nil {
		cc = *countryCode
	}
	mobile = strings.TrimSpace(mobile)
	if mobile == "" {
		return ""
	}

	hasPlus := mobile[0] == '+'
	var b strings.Builder
	b.Grow(len(mobile))
	for _, r := range mobile {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	if cleaned == "" {
		return ""
	}

	if hasPlus {
		mobile = "+" + cleaned
	} else if len(cleaned) > 1 {
		mobile = cc + cleaned[1:]
	} else {
		mobile = cc
	}

	mobile = strings.Replace(mobile, "+6208", "+628", 1)
	return mobile
}

var auditFieldNames = map[string]struct{}{
	"Status":    {},
	"CreatedBy": {},
	"CreatedAt": {},
	"UpdatedBy": {},
	"UpdatedAt": {},
}

func isAuditField(name string, stripFields map[string]struct{}) bool {
	_, ok := stripFields[name]
	return ok
}

func ToPascalCase(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	if strings.Contains(name, "_") {
		parts := strings.Split(name, "_")
		var b strings.Builder
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			b.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				b.WriteString(strings.ToLower(part[1:]))
			}
		}

		return b.String()
	}

	return strings.ToUpper(name[:1]) + name[1:]
}

func buildStripFieldNames(optionalColumns []string) map[string]struct{} {
	stripFields := make(map[string]struct{}, len(auditFieldNames)+len(optionalColumns))

	for fieldName := range auditFieldNames {
		stripFields[fieldName] = struct{}{}
	}

	for _, fieldName := range optionalColumns {
		pascalFieldName := ToPascalCase(fieldName)
		if pascalFieldName == "" {
			continue
		}

		stripFields[pascalFieldName] = struct{}{}
	}

	return stripFields
}

// StripCommonAuditFields clears common audit pointer fields (Status, CreatedBy, CreatedAt, UpdatedBy, UpdatedAt)
// anywhere inside the struct graph of ptr (including deeply embedded / nested structs).
func StripCommonAuditFields[T any](ptr *T, optionalColumns ...string) {
	if ptr == nil {
		return
	}

	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}

	// visited keyed by pointer address to break cycles in pointer graphs
	visited := make(map[uintptr]struct{})
	stripFields := buildStripFieldNames(optionalColumns)

	stripCommonAuditFieldsValue(v.Elem(), visited, stripFields)
}

// StripCommonAuditFieldsSlice applies StripCommonAuditFields to each element in the slice.
func StripCommonAuditFieldsSlice[T any](items []T, optionalColumns ...string) {
	for i := range items {
		StripCommonAuditFields(&items[i], optionalColumns...)
	}
}

// internal: recursively walk structs / pointer-to-struct and nil audit fields by name.
func stripCommonAuditFieldsValue(v reflect.Value, visited map[uintptr]struct{}, stripFields map[string]struct{}) {
	if !v.IsValid() {
		return
	}

	// Allow both struct and *struct, with cycle protection for pointers.
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}

		ptr := v.Pointer()
		if _, seen := visited[ptr]; seen {
			// already processed this node, break cycle
			return
		}
		visited[ptr] = struct{}{}

		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		ft := t.Field(i)

		// If this field *itself* is an audit field pointer: nil it.
		if isAuditField(ft.Name, stripFields) && fv.CanSet() && fv.Kind() == reflect.Ptr {
			fv.Set(reflect.Zero(fv.Type()))
			continue
		}

		// Recurse into nested/embedded structs
		switch fv.Kind() {
		case reflect.Struct, reflect.Ptr:
			stripCommonAuditFieldsValue(fv, visited, stripFields)
		}
	}
}
