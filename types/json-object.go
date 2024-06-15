package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

type JsonObject struct {
	Raw  json.RawMessage
	Data map[string]any
}

func NewJsonObject(data map[string]any) JsonObject {
	raw, _ := json.Marshal(data)
	return JsonObject{Data: data, Raw: raw}
}

func NewJsonObjectFromString(dt string) (res JsonObject, err error) {
	err = res.UnmarshalJSON([]byte(dt))
	return res, err
}

//goland:noinspection GoMixedReceiverTypes
func (t *JsonObject) Scan(src any) error {
	if v, ok := src.([]byte); ok {
		return t.UnmarshalJSON(v)
	} else {
		return fmt.Errorf("error scanning %v into JsonObject.Raw", src)
	}
}

//goland:noinspection GoMixedReceiverTypes
func (t *JsonObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Data)
}

// Value implements the driver Valuer interface.
//
//goland:noinspection GoMixedReceiverTypes
func (t JsonObject) Value() (driver.Value, error) {
	return []byte(t.Raw), nil
}

// UnmarshalJSON implement json decoder
//
//goland:noinspection GoMixedReceiverTypes
func (t *JsonObject) UnmarshalJSON(bytes []byte) error {
	var (
		data = map[string]any{}
	)

	err := json.Unmarshal(bytes, &data)
	if err != nil {
		return err
	}

	t.Raw = bytes
	t.Data = data
	return nil
}

// RegisterCustomTypeFunc digunakan utk decode dari form data menjadi JsonObject
//
//goland:noinspection GoMixedReceiverTypes
func (t JsonObject) RegisterCustomTypeFunc() func(vals []string) (i interface{}, e error) {
	return func(vals []string) (i interface{}, e error) {
		r := JsonObject{}
		if err := r.UnmarshalJSON([]byte(vals[0])); err != nil {
			return nil, err
		}
		return r, nil
	}
}

// ValidateRegisterCustomTypeFunc dipakai untuk struct validator
//
//goland:noinspection GoMixedReceiverTypes
func (t JsonObject) ValidateRegisterCustomTypeFunc(field reflect.Value) interface{} {
	if rp, ok := field.Interface().(JsonObject); ok {
		return rp.Data
	} else if rp, ok := field.Interface().(*JsonObject); ok && rp != nil {
		return rp.Data
	}
	return nil
}

// EncodeCustomTypeFunc digunakan utk endcode dari tipe ini ke form data format
func (t JsonObject) EncodeCustomTypeFunc() func(x interface{}) ([]string, error) {
	return func(x interface{}) ([]string, error) {
		return []string{string(x.(JsonObject).Raw)}, nil
	}
}
