package types

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type NativeDate struct {
	time.Time
}

func NewNativeDate(dt time.Time) NativeDate {
	return NativeDate{Time: dt}
}

func NewNativeDateFromString(dt string) (res NativeDate, err error) {
	err = res.UnmarshalJSON([]byte(dt))
	return res, err
}

//goland:noinspection GoMixedReceiverTypes
func (t *NativeDate) Scan(src any) error {
	if v, ok := src.(time.Time); ok {
		t.Time = v
	} else if v, ok := src.([]byte); ok {
		return t.UnmarshalJSON(v)
	} else {
		return fmt.Errorf("error scanning %v into NativeDate.time.Time", src)
	}
	return nil
}

//goland:noinspection GoMixedReceiverTypes
func (t *NativeDate) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.Format("2006-01-02") + `"`), nil
}

//goland:noinspection GoMixedReceiverTypes
func (t NativeDate) String() string {
	return t.Format("2006-01-02")
}

// Value implements the driver Valuer interface.
//
//goland:noinspection GoMixedReceiverTypes
func (t NativeDate) Value() (driver.Value, error) {
	return t.Time, nil
}

// UnmarshalJSON implement json decoder
//
//goland:noinspection GoMixedReceiverTypes
func (t *NativeDate) UnmarshalJSON(bytes []byte) error {
	var (
		data string
		Len  = len(bytes)
	)
	if Len == 0 {
		return errors.New("NativeDate invalid parameter")
	} else if Len > 0 && bytes[0] == '"' {
		data = strings.Trim(string(bytes), `"`)
	} else {
		data = string(bytes)
	}
	Date, err := time.Parse("2006-01-02", data)
	if err != nil {
		return err
	}
	t.Time = Date
	return nil
}

// RegisterCustomTypeFunc digunakan utk decode dari form data menjadi NativeDate
//
//goland:noinspection GoMixedReceiverTypes
func (t NativeDate) RegisterCustomTypeFunc() func(vals []string) (i interface{}, e error) {
	return func(vals []string) (i interface{}, e error) {
		r := NativeDate{}
		if err := r.UnmarshalJSON([]byte(vals[0])); err != nil {
			return nil, err
		}
		return r, nil
	}
}

// ValidateRegisterCustomTypeFunc dipakai untuk struct validator
//
//goland:noinspection GoMixedReceiverTypes
func (t NativeDate) ValidateRegisterCustomTypeFunc(field reflect.Value) interface{} {
	if rp, ok := field.Interface().(NativeDate); ok {
		return rp.Time
	} else if rp, ok := field.Interface().(*NativeDate); ok && rp != nil {
		return rp.Time
	}
	return nil
}
