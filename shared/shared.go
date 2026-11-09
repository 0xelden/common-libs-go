package shared

import (
	"database/sql"
	"database/sql/driver"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/0xelden/common-libs-go/types"
	"github.com/go-playground/validator/v10"
	"github.com/gookit/validate/v2"
	_ "github.com/joho/godotenv/autoload"
)

var (
	// Validate returns A new instance of 'validate' with sane defaults.
	// Validate is designed to be thread-safe and used as A singleton instance.
	// It caches information about your struct and validations,
	// in essence only parsing your validation tags once per struct type.
	// Using multiple instances neglects the benefit of caching.
	Validate = validator.New()

	Staging = os.Getenv("APP_ENV") == "staging" || os.Getenv("APP_ENV") == "local"
	Tracing = len(os.Getenv("UPTRACE_DSN")) > 0
)

func init() {
	// validate/v2 only descends into a named sub-struct whose field carries a
	// validate tag; keep v1's always-cascade so existing models (which embed
	// their Dto and nest structs untagged) keep every rule they had.
	validate.Config(func(o *validate.GlobalOption) {
		o.CheckSubOnParentMarked = false
	})

	_ = Validate.RegisterValidation("dtvalue", func(fl validator.FieldLevel) bool {
		switch typ := fl.Field().Interface().(type) {
		case *time.Time:
			return typ != nil && !typ.IsZero()
		case time.Time:
			return !typ.IsZero()
		default:
			return false
		}
	})
	Validate.RegisterCustomTypeFunc(
		ValidateValuer, sql.NullString{}, sql.NullInt64{}, sql.NullBool{},
		sql.NullFloat64{}, sql.NullTime{}, sql.NullInt32{}, sql.NullByte{},
	)
	Validate.RegisterCustomTypeFunc(
		(&types.RupiahCent{}).ValidateRegisterCustomTypeFunc,
		types.RupiahCent{},
		&types.RupiahCent{},
	)
	Validate.RegisterCustomTypeFunc(
		(&types.NativeDate{}).ValidateRegisterCustomTypeFunc,
		types.NativeDate{},
		&types.NativeDate{},
	)
}

// ValidateValuer implements validator.CustomTypeFunc
func ValidateValuer(field reflect.Value) interface{} {
	if valuer, ok := field.Interface().(driver.Valuer); ok {
		val, err := valuer.Value()
		if err == nil {
			return val
		}
	}
	return nil
}

func ValidateStruct(scene string, s any) error {
	v := validate.Struct(s)
	if fields := v.AtScene(scene).SceneFields(); len(fields) > 0 {
		v.WithScenes(validate.SValues{scene: withEmbeddedScenePaths(reflect.TypeOf(s), fields)})
	}
	if v.Validate(scene) {
		return nil
	}
	return v.Errors
}

// withEmbeddedScenePaths adds the embedded path of every scene field that is
// promoted from an embedded struct. validate registers such a rule under
// "<Embedded>.<Field>" (e.g. "UserDto.Email"), while ConfigValidation scenes
// name the promoted field ("Email"), so the scene filter never matched and
// the rule was silently skipped.
func withEmbeddedScenePaths(t reflect.Type, fields []string) []string {
	embedded := embeddedStructs(t, "")
	if len(embedded) == 0 {
		return fields
	}
	out := append([]string(nil), fields...)
	for _, field := range fields {
		head, _, _ := strings.Cut(field, ".")
		for path, et := range embedded {
			if hasDirectField(et, head) {
				out = append(out, path+"."+field)
			}
		}
	}
	return out
}

// embeddedStructs maps the dotted path of every (possibly nested) embedded
// struct in t to its type.
func embeddedStructs(t reflect.Type, prefix string) map[string]reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	found := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous {
			continue
		}
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() != reflect.Struct {
			continue
		}
		path := f.Name
		if prefix != "" {
			path = prefix + "." + f.Name
		}
		found[path] = ft
		for p, nt := range embeddedStructs(ft, path) {
			found[p] = nt
		}
	}
	return found
}

func hasDirectField(t reflect.Type, name string) bool {
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); !f.Anonymous && f.Name == name {
			return true
		}
	}
	return false
}
