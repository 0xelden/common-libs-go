package types

import (
	"database/sql/driver"
	"reflect"

	"github.com/jackc/pgtype"
)

type NativeTime struct {
	pgtype.Time
}

func NewNativeTimeFromString(Time string) (res NativeTime, err error) {
	err = res.UnmarshalJSON([]byte(Time))
	return res, err
}

//goland:noinspection GoMixedReceiverTypes
func (t *NativeTime) Scan(src any) error {
	return t.Time.Scan(src)
}

//goland:noinspection GoMixedReceiverTypes
func (t *NativeTime) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte(`""`), nil
	}
	txt, err := t.Time.EncodeText(nil, []byte{})
	if err != nil {
		return nil, err
	}
	if len(txt) < 8 {
		return []byte(`""`), nil
	}
	return append(append([]byte(`"`), txt[:8]...), byte('"')), nil
}

//goland:noinspection GoMixedReceiverTypes
func (t NativeTime) String() string {
	txt, _ := t.Time.EncodeText(nil, []byte{})
	if len(txt) < 8 {
		return ""
	}
	return string(txt[:8])
}

// Value implements the driver Valuer interface.
//
//goland:noinspection GoMixedReceiverTypes
func (t NativeTime) Value() (driver.Value, error) {
	return t.Time.Value()
}

// UnmarshalJSON implement json decoder
//
//goland:noinspection GoMixedReceiverTypes
func (t *NativeTime) UnmarshalJSON(bytes []byte) error {
	if len(bytes) > 1 && string(bytes[0]) == `"` {
		bytes = bytes[1 : len(bytes)-1]
	}
	return t.Time.DecodeText(nil, bytes)
}

// RegisterCustomTypeFunc digunakan utk decode dari form data menjadi NativeTime
//
//goland:noinspection GoMixedReceiverTypes
func (t NativeTime) RegisterCustomTypeFunc() func(vals []string) (i interface{}, e error) {
	return func(vals []string) (i interface{}, e error) {
		r := NativeTime{}
		if err := r.UnmarshalJSON([]byte(vals[0])); err != nil {
			return nil, err
		}
		return r, nil
	}
}

// ValidateRegisterCustomTypeFunc dipakai untuk struct validator
//
//goland:noinspection GoMixedReceiverTypes
func (t NativeTime) ValidateRegisterCustomTypeFunc(field reflect.Value) interface{} {
	if rp, ok := field.Interface().(NativeTime); ok {
		return rp.Time
	} else if rp, ok := field.Interface().(*NativeTime); ok && rp != nil {
		return rp.Time
	}
	return nil
}
