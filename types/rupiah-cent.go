package types

import (
	"database/sql/driver"
	"fmt"
	"reflect"

	"github.com/shopspring/decimal"
)

// RupiahCent tipe khusus untuk simpan satuan rupiah dalam cent
// 1 rupiah adalah 100 cent
// sudah implementasi standard sql Scan
// implementasi MarshalJSON & MarshalJSON sudah ada
type RupiahCent struct {
	Int64 int64
}

func NewRupiahCent(int64 int64) RupiahCent {
	return RupiahCent{Int64: int64}
}

func NewRupiahCentFromDecimal(dec decimal.Decimal) RupiahCent {
	// Extract the integer part of the decimal
	intPart := dec.Truncate(0)

	// Subtract the integer part from the original to get the decimal part
	decimalPart := dec.Sub(intPart)

	// we use 100 because 1 rupiah is consisting of 100 cent
	_100 := decimal.NewFromInt(100)
	decimalDigits := decimalPart.Mul(_100).Truncate(0)
	return RupiahCent{
		Int64: intPart.Mul(_100).Add(decimalDigits).IntPart(),
	}
}

func NewRupiahCentFromFloat(float64 float64) RupiahCent {
	d := decimal.NewFromFloat(float64)
	return NewRupiahCentFromDecimal(d)
}

// Value implements the driver Valuer interface.
//
//goland:noinspection GoMixedReceiverTypes
func (t RupiahCent) Value() (driver.Value, error) {
	return t.Int64, nil
}

// Scan implements the Scanner interface.
//
//goland:noinspection GoMixedReceiverTypes
func (t *RupiahCent) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		t.Int64 = v
	case []uint8:
		d, err := decimal.NewFromString(string(v))
		if err != nil {
			return err
		}
		t.Int64 = d.BigInt().Int64()
	case int:
		t.Int64 = int64(v)
	case int8:
		t.Int64 = int64(v)
	case int16:
		t.Int64 = int64(v)
	case int32:
		t.Int64 = int64(v)
	case float32:
		t.Int64 = decimal.NewFromFloat(float64(v)).BigInt().Int64()
	case float64:
		t.Int64 = decimal.NewFromFloat(v).BigInt().Int64()
	case string:
		d, err := decimal.NewFromString(v)
		if err != nil {
			return err
		}
		t.Int64 = d.BigInt().Int64()
	case nil:
		t.Int64 = 0
	default:
		return fmt.Errorf("error scanning %v into RupiahCent.Scan", src)
	}
	return nil
}

// UnmarshalJSON implement json decoder
//
//goland:noinspection GoMixedReceiverTypes
func (t *RupiahCent) UnmarshalJSON(bytes []byte) error {
	d, err := decimal.NewFromString(string(bytes))
	if err != nil {
		return err
	}
	t.Int64 = NewRupiahCentFromDecimal(d).Int64
	return nil
}

// MarshalJSON https://stackoverflow.com/a/58088873
//
//goland:noinspection GoMixedReceiverTypes
func (t RupiahCent) MarshalJSON() ([]byte, error) {
	//return []byte(p.Sprintf(`"Rp. %d,%02d"`, t.Time/100, int64(math.Abs(float64(t.Time%100))))), nil
	return []byte(fmt.Sprintf(`%d`, t.Int64)), nil
}

// RegisterCustomTypeFunc digunakan utk decode dari form data menjadi RupiahCent
//
//goland:noinspection GoMixedReceiverTypes
func (t RupiahCent) RegisterCustomTypeFunc() func(vals []string) (i interface{}, e error) {
	return func(vals []string) (i interface{}, e error) {
		r := RupiahCent{}
		if err := r.UnmarshalJSON([]byte(vals[0])); err != nil {
			return nil, err
		}
		return r, nil
	}
}

// ValidateRegisterCustomTypeFunc dipakai untuk struct validator
//
//goland:noinspection GoMixedReceiverTypes
func (t RupiahCent) ValidateRegisterCustomTypeFunc(field reflect.Value) interface{} {
	if rp, ok := field.Interface().(RupiahCent); ok {
		return rp.Int64
	} else if rp, ok := field.Interface().(*RupiahCent); ok && rp != nil {
		return rp.Int64
	}
	return nil
}
