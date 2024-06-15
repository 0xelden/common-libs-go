package helper

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	_ "github.com/joho/godotenv/autoload"
	"github.com/uptrace/uptrace-go/uptrace"
	"github.com/0xelden/common-libs-go/serror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// ContextGetString digunakan untuk parse value dari context dengan aman
// support context value yg dikirim dari grpc client ke grpc server
func ContextGetString(ctx context.Context, key string, defaults ...string) string {
	if val := ctx.Value(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	if val := metadata.ValueFromIncomingContext(ctx, key); len(val) > 0 {
		return val[len(val)-1] // last one win
	}
	for _, val := range defaults {
		if len(val) > 0 {
			return val
		}
	}
	return ""
}

// ContextGetAny mirip ContextGetString tapi mengguanakn any type
func ContextGetAny(ctx context.Context, key string, defaults ...any) any {
	if val := ctx.Value(key); val != nil {
		return val
	}
	if val := metadata.ValueFromIncomingContext(ctx, key); len(val) > 0 {
		return val[len(val)-1] // last one win
	}
	for _, val := range defaults {
		return val
	}
	return nil
}

// ContextSetString set key value string pair to standard context
func ContextSetString(ctx context.Context, key, value string, pair ...string) context.Context {
	ctx = context.WithValue(ctx, key, value)
	for i := 0; i < len(pair); i += 2 {
		ctx = context.WithValue(ctx, pair[i], pair[i+1])
	}
	return ctx
}

// GrpcContextSetString set key value string ke grpc context dan http context
func GrpcContextSetString(ctx context.Context, kv ...string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, kv...)
}

// RecordParam record a param to a tracer
// Deprecated: on a same context, multiple call to RecordParam only write the last one because of the same attribute name
// use Trace instead
func RecordParam(ctx context.Context, param interface{}) trace.Span {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		rand := RandStringUnsafe(5)
		data, _ := json.MarshalIndent(param, "", "\t")
		span.SetAttributes(attribute.String("base.param."+rand, string(data)))
	}
	return span
}

// Trace record json encoded value with key attribute to a tracer, use TraceUnique to enforce key uniqueness using random key suffix
func Trace(ctx context.Context, kvPairs ...any) trace.Span {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() || len(kvPairs) == 0 {
		return span
	}

	for i := 0; i < len(kvPairs); i += 2 {
		if i+1 >= len(kvPairs) {
			break
		}
		key, ok := kvPairs[i].(string)
		if !ok || len(key) == 0 {
			continue
		}
		value := kvPairs[i+1]
		data, _ := json.MarshalIndent(value, "", "\t")
		span.SetAttributes(attribute.String(key, string(data)))
	}

	return span
}

func TraceUnique(ctx context.Context, kvPairs ...any) trace.Span {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() || len(kvPairs) == 0 {
		return span
	}

	rand := RandStringUnsafe(5)

	// We process pairs in steps of 2: (key, value).
	for i := 0; i < len(kvPairs); i += 2 {
		// If there aren’t enough items left for a pair, break out.
		if i+1 >= len(kvPairs) {
			break
		}

		key, ok := kvPairs[i].(string)
		if !ok || len(key) == 0 {
			continue
		}

		value := kvPairs[i+1]
		data, _ := json.MarshalIndent(value, "", "\t")
		span.SetAttributes(attribute.String(
			key+"."+rand,
			string(data),
		))
	}

	return span
}

// RecordErrorTrace record a serr stacktrace to tracer
func RecordErrorTrace(ctx context.Context, serr serror.SError) trace.Span {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		stack := Reduce(serr.StackFrames(), []map[string]any{}, func(res []map[string]any, val errors.StackFrame, i int) []map[string]any {
			return append(res, map[string]any{
				"file": val.File,
				"line": val.LineNumber,
				"pkg":  val.Package,
			})
		})
		if length := len(stack); length > 4 {
			stack = stack[:4]
		}
		span.RecordError(serr)
		span.SetAttributes(attribute.String("base.stacktrace", string(First(json.MarshalIndent(stack, "", "\t")))))
	}
	return span
}

// StructToMap mengubah struct menjadi map, semua field dipilih atau isi fields arg dan berubah menjadi selective
func StructToMap(data any, structTag string, fields ...string) map[string]any {
	fmaps := make(map[string]struct{}, len(fields))
	selective := len(fields) > 0
	if selective {
		for _, f := range fields {
			fmaps[f] = struct{}{}
		}
	}
	out := make(map[string]any)
	v := reflect.ValueOf(data)
	if !v.IsValid() {
		return out
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return out // nil ptr – nothing to map
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return out // unsupported input
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		// If the field is unexported, CanInterface() == false.
		if !v.Field(i).CanInterface() {
			continue // or handle it some other way
		}

		field := t.Field(i)
		tag := structFieldTag(field, structTag)
		// ignore "-" and strip ",options"
		if tag == "-" {
			continue
		}
		if comma := strings.IndexByte(tag, ','); comma != -1 {
			tag = tag[:comma]
		}

		ifc := v.Field(i).Interface()
		//if val := reflect.ValueOf(ifc); val.Kind() == reflect.Ptr && val.IsNil() {
		//	continue
		//}

		// handle embedded field
		if field.Anonymous {
			for k, v2 := range StructToMap(ifc, structTag, fields...) {
				if !selective {
					out[k] = v2
				} else {
					if _, ok := fmaps[tag]; ok {
						out[k] = v2
					}
				}
			}
			continue
		}

		if !selective {
			out[tag] = ifc
		} else {
			if _, ok := fmaps[tag]; ok {
				out[tag] = ifc
			}
		}
	}
	return out
}

// StructToMapDB mengubah struct menjadi map, jika struct memiliki embedded field, hanya embedded field pertama yg diambil
func StructToMapDB(data any, fields ...string) map[string]any {
	fmaps := make(map[string]struct{}, len(fields))
	selective := len(fields) > 0
	if selective {
		for _, f := range fields {
			fmaps[f] = struct{}{}
		}
	}
	out := make(map[string]any)
	v := reflect.ValueOf(data)
	if !v.IsValid() {
		return out
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return out // nil ptr – nothing to map
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return out // unsupported input
	}
	t := v.Type()
	structTag := "db"
	isEmbedded := false
	for i := 0; i < v.NumField(); i++ {
		if t.Field(i).Anonymous {
			isEmbedded = true
			break
		}
	}
	if !isEmbedded {
		return StructToMap(data, structTag, fields...)
	}
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		tag := structFieldTag(field, structTag)
		// ignore "-" and strip ",options"
		if tag == "-" {
			continue
		}
		if comma := strings.IndexByte(tag, ','); comma != -1 {
			tag = tag[:comma]
		}

		// handle embedded field
		if field.Anonymous {
			ifc := v.Field(i).Interface()
			for k, v2 := range StructToMap(ifc, structTag, fields...) {
				if !selective {
					out[k] = v2
				} else {
					if _, ok := fmaps[tag]; ok {
						out[k] = v2
					}
				}
			}
			break
		}
	}
	return out
}

func structFieldTag(field reflect.StructField, structTag string) string {
	if tag := field.Tag.Get(structTag); tag != "" {
		return tag
	}
	if structTag == "db" {
		if tag := field.Tag.Get("json"); tag != "" {
			return tag
		}
		if tag := field.Tag.Get("form"); tag != "" {
			return tag
		}
	}
	return field.Name
}

// StructToJsonMap mirip StructToMap yang menggunakan json tag secara default
func StructToJsonMap(data any, fields ...string) (res map[string]any, err error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	mapData := make(map[string]any)
	err = json.Unmarshal(dataBytes, &mapData)
	if err != nil {
		return nil, err
	}

	if len(fields) == 0 {
		return mapData, nil
	}

	res = make(map[string]any, len(fields))
	for _, f := range fields {
		val, ok := mapData[f]
		if !ok {
			continue
		}
		res[f] = val
	}

	return res, nil
}

// DeterministicUUID membuat uuid dari source, jika source a maka balikan uuid selalu b, deterministic
func DeterministicUUID(source string) (string, error) {
	// calculate the MD5 hash of the source
	md5hash := md5.New()
	md5hash.Write([]byte(source))

	// convert the hash value to a string
	md5string := hex.EncodeToString(md5hash.Sum(nil))

	// generate the UUID from the
	// first 16 bytes of the MD5 hash
	id, err := uuid.FromBytes([]byte(md5string[0:16]))
	if err != nil {
		return "", err
	}

	return id.String(), nil
}

func StrToTitleCase(input string) string {
	words := strings.Split(input, " ")
	smallwords := " a an on the to "
	for index, word := range words {
		if strings.Contains(smallwords, " "+word+" ") && word != string(word[0]) {
			words[index] = word
		} else {
			words[index] = strings.Title(word)
		}
	}
	return strings.Join(words, " ")
}

func GitCommitID() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func ToJsonIndent(data any) string {
	res, _ := json.MarshalIndent(data, "", "\t")
	return string(res)
}

func ToJson(data any) string {
	res, _ := json.MarshalIndent(data, "", "")
	return string(res)
}


func TraceURL(span trace.Span) string {
	addr := Env("UPTRACE_WEB")
	if _, err := url.Parse(addr); err != nil {
		addr = ""
	}
	if addr != "" {
		return fmt.Sprintf("%s/traces/%s", addr, span.SpanContext().TraceID())
	}
	return uptrace.TraceURL(span)
}

func ToEnvKey(str string) string {
	return strings.TrimSpace(strings.ToUpper(strings.ReplaceAll(str, ":", "_")))
}

// JoinPath joins a base URL with multiple path components.
func JoinPath(base string, comps ...string) string {
	parsedBase, err := url.Parse(base)
	if err != nil || parsedBase == nil {
		u := strings.TrimRight(base, "/")
		for _, c := range comps {
			c = strings.Trim(c, "/")
			if c == "" {
				continue
			}
			if u == "" {
				u = c
				continue
			}
			u += "/" + c
		}
		return u
	}
	joinedPath := path.Join(append([]string{parsedBase.Path}, comps...)...)
	parsedBase.Path = joinedPath
	return parsedBase.String()
}


// NormalizedEnvLower returns the lower-case value of an environment
// variable built from a base key and a suffix.
//
// Construction rules:
//
//  1. Compose the raw key by concatenating baseKey and suffix with “_”,
//     e.g. baseKey="task:notif:email", suffix="mock" ➜ "task:notif:email_mock".
//  2. Convert that raw key to canonical ENV-KEY style via helper.ToEnvKey
//     (→ "TASK_NOTIF_EMAIL_MOCK").
//  3. Look up the variable in the process environment with helper.Env.
//  4. Lower-case the result with helper.NormalizeLower, falling back to the
//     empty string when unset.
func NormalizedEnvLower(baseKey, suffix string) string {
	return NormalizeLower(Env(ToEnvKey(baseKey+If(len(suffix) > 0, "_"+suffix, ""))), "")
}

// GreetingByTime returns a greeting based on the hour of the given time.
func GreetingByTime(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 1 && hour < 10:
		return "Selamat pagi"
	case hour >= 10 && hour < 14:
		return "Selamat siang"
	case hour >= 14 && hour < 18:
		return "Selamat sore"
	default:
		return "Selamat malam"
	}
}

func UrlOrigin(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid URL: %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}
