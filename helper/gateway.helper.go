package helper

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rivo/uniseg"
)

func IntToString(v int) string {
	return Int64ToString(int64(v))
}

func Int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func BoolToString(v bool) string {
	return strconv.FormatBool(v)
}

func Length(s string) int64 {
	return int64(uniseg.GraphemeClusterCount(s))
}

func LeftPad(s string, l int, p string) string {
	for i := Length(s); i < int64(l); i++ {
		s = p + s
	}
	return s
}

func Sub(s string, f int, l int) string {
	if l == 0 {
		l = len(s) - f
	} else if l < 0 {
		l = len(s) + l
	}

	return s[f : f+l]
}

func Env(key string, def ...string) string {
	return Chains(append([]string{os.Getenv(key)}, def...)...)
}

func StrIndex(s string, value string, offset int) int {
	if offset > 0 {
		s = Sub(s, offset, 0)
	}

	if offset < 0 {
		offset = 0
	}

	i := strings.Index(s, value)
	if i >= 0 {
		return i + offset
	}
	return i
}

func Trim(value string) string {
	return strings.TrimSpace(value)
}

func Chains(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func MD5(val string) string {
	mdcp := md5.New()

	_, err := mdcp.Write([]byte(val))
	if err != nil {
		return ""
	}

	return hex.EncodeToString(mdcp.Sum(nil))
}

func SHA1(val string) string {
	sha := sha1.New()
	_, err := sha.Write([]byte(val))
	if err != nil {
		return ""
	}

	return hex.EncodeToString(sha.Sum(nil))
}

func CleanSpit(value string, sep string) []string {
	values := strings.Split(value, sep)
	for k, v := range values {
		values[k] = Trim(v)
	}
	return values
}

func StringToInt(value string, def int64) int64 {
	r, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		r = def
	}
	return r
}

func StringToFloat(value string, def float64) float64 {
	r, err := strconv.ParseFloat(value, 64)
	if err != nil {
		r = def
	}
	return r
}

func IsInteger(v string) bool {
	if v == "" {
		return false
	}

	a := "1234567890"
	for _, v := range v {
		if !strings.Contains(a, string(v)) {
			return false
		}
	}

	return true
}

func IsExists(p string) bool {
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		return true
	}
	return false
}

func IsNil(value interface{}) (res bool) {
	return value == nil || (reflect.TypeOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil())
}

func IsZero(value interface{}) (res bool) {
	res = true

	if !reflect.ValueOf(value).IsNil() {
		res = false
	}
	return
}

func ToString(value interface{}) (res string) {
	if !IsNil(value) {
		// Check if value implements fmt.Stringer
		if stringer, ok := value.(fmt.Stringer); ok {
			return stringer.String()
		}
		val := reflect.ValueOf(value)
		switch val.Kind() {
		case reflect.String:
			res = val.String()

		case reflect.Ptr:
			res = ToString(reflect.Indirect(val))

		default:
			byt, err := json.Marshal(value)
			if err == nil {
				res = string(byt)
			}
		}
	}
	return
}

func Clone(value interface{}) interface{} {
	val := reflect.ValueOf(value)
	typ := reflect.TypeOf(value)

	ptr := false
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
		typ = reflect.TypeOf(val.Interface())
		ptr = true
	}

	nval := reflect.New(typ)
	if ptr {
		nval.Elem().Set(reflect.ValueOf(val.Interface()))
	} else {
		nval = reflect.Indirect(nval)
		nval.Set(reflect.ValueOf(val.Interface()))
	}
	return nval.Interface()
}

func ToBool(value interface{}, def bool) bool {
	vx := reflect.ValueOf(value)
	switch vx.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int, reflect.Int64:
		switch vx.Int() {
		case 1:
			return true

		case 0:
			return false
		}

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint, reflect.Uint64:
		switch vx.Uint() {
		case 1:
			return true

		case 0:
			return false
		}

	case reflect.Bool:
		return vx.Bool()

	default:
		switch strings.ToLower(ToString(value)) {
		case "true", "1":
			return true

		case "false", "0":
			return false
		}
	}

	return def
}
