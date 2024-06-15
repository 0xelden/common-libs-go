package shared

import (
	"database/sql"
	"database/sql/driver"
	"os"
	"reflect"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gookit/validate"
	_ "github.com/joho/godotenv/autoload"
	"github.com/0xelden/common-libs-go/types"
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
	if v.Validate(scene) {
		return nil
	}
	return v.Errors
}
