package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

type JsonArray struct {
	Raw  json.RawMessage
	Data []any
}

func NewJsonArray(data []any) JsonArray {
	raw, _ := json.Marshal(data)
	return JsonArray{Data: data, Raw: raw}
}

func NewJsonArrayFromString(dt string) (res JsonArray, err error) {
	err = res.UnmarshalJSON([]byte(dt))
	return res, err
}

//goland:noinspection GoMixedReceiverTypes
func (t *JsonArray) Scan(src any) error {
	if v, ok := src.([]byte); ok {
		return t.UnmarshalJSON(v)
	} else {
		return fmt.Errorf("error scanning %v into JsonArray.Raw", src)
	}
}

//goland:noinspection GoMixedReceiverTypes
func (t *JsonArray) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Data)
}

// Value implements the driver Valuer interface.
//
//goland:noinspection GoMixedReceiverTypes
func (t JsonArray) Value() (driver.Value, error) {
	return []byte(t.Raw), nil
}

// UnmarshalJSON implement json decoder
//
//goland:noinspection GoMixedReceiverTypes
func (t *JsonArray) UnmarshalJSON(bytes []byte) error {
	var (
		data = []any{}
	)

	err := json.Unmarshal(bytes, &data)
	if err != nil {
		return err
	}

	t.Raw = bytes
	t.Data = data
	return nil
}

// RegisterCustomTypeFunc digunakan utk decode dari form data menjadi JsonArray
//
//goland:noinspection GoMixedReceiverTypes
func (t JsonArray) RegisterCustomTypeFunc() func(vals []string) (i interface{}, e error) {
	return func(vals []string) (i interface{}, e error) {
		r := JsonArray{}
		if err := r.UnmarshalJSON([]byte(vals[0])); err != nil {
			return nil, err
		}
		return r, nil
	}
}

// EncodeCustomTypeFunc digunakan utk endcode dari tipe ini ke form data format
func (t JsonArray) EncodeCustomTypeFunc() func(x interface{}) ([]string, error) {
	return func(x interface{}) ([]string, error) {
		return []string{string(x.(JsonArray).Raw)}, nil
	}
}

// ValidateRegisterCustomTypeFunc dipakai untuk struct validator
//
//goland:noinspection GoMixedReceiverTypes
func (t JsonArray) ValidateRegisterCustomTypeFunc(field reflect.Value) interface{} {
	if rp, ok := field.Interface().(JsonArray); ok {
		return rp.Data
	} else if rp, ok := field.Interface().(*JsonArray); ok && rp != nil {
		return rp.Data
	}
	return nil
}
