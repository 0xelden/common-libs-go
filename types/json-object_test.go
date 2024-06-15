package types

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/go-playground/form"
	. "gopkg.in/go-playground/assert.v1"
)

func TestJsonObject_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		fields  map[string]any
		want    []byte
		wantErr bool
	}{
		{
			name: "Simple Object",
			fields: map[string]any{
				"key": "value",
			},
			want:    []byte(`{"key":"value"}`),
			wantErr: false,
		},
		{
			name: "Nested Object",
			fields: map[string]any{
				"key": map[string]any{"nestedKey": "nestedValue"},
			},
			want:    []byte(`{"key":{"nestedKey":"nestedValue"}}`),
			wantErr: false,
		},
		{
			name:    "Empty Object",
			fields:  map[string]any{},
			want:    []byte(`{}`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := JsonObject{Data: tt.fields}
			got, err := obj.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarshalJSON() got = %s, want %s", string(got), string(tt.want))
			}
		})
	}
}

func TestJsonObject_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "Valid Object",
			input:   []byte(`{"key":"value"}`),
			want:    map[string]any{"key": "value"},
			wantErr: false,
		},
		{
			name:    "Nested Object",
			input:   []byte(`{"key":{"nestedKey":"nestedValue"}}`),
			want:    map[string]any{"key": map[string]any{"nestedKey": "nestedValue"}},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   []byte(`{"key":`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &JsonObject{}
			err := obj.UnmarshalJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(obj.Data, tt.want) {
				t.Errorf("UnmarshalJSON() got = %v, want %v", obj.Data, tt.want)
			}
		})
	}
}

func TestJsonObject_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "Valid JSON Bytes",
			input:   []byte(`{"key":"value"}`),
			want:    map[string]any{"key": "value"},
			wantErr: false,
		},
		{
			name:    "Invalid JSON Bytes",
			input:   []byte(`{"key":`),
			wantErr: true,
		},
		{
			name:    "Unsupported Type",
			input:   "string",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &JsonObject{}
			err := obj.Scan(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(obj.Data, tt.want) {
				t.Errorf("Scan() got = %v, want %v", obj.Data, tt.want)
			}
		})
	}
}

func TestJsonObjectDecoder_RegisterCustomTypeFunc(t *testing.T) {
	type TestStruct struct {
		Object       JsonObject  `form:"object"`
		ObjectPtr    *JsonObject `form:"object_ptr"`
		ObjectPtrNil *JsonObject `form:"object_ptr_nil"`
	}

	d := form.NewDecoder()
	d.RegisterCustomTypeFunc((&JsonObject{}).RegisterCustomTypeFunc(), JsonObject{})

	var v TestStruct
	err := d.Decode(&v, url.Values{
		"object":     []string{`{"key":"value"}`},
		"object_ptr": []string{`{"nestedKey":"nestedValue"}`},
	})

	Equal(t, err, nil)
	Equal(t, v.Object.Data, map[string]any{"key": "value"})
	Equal(t, v.ObjectPtr.Data, map[string]any{"nestedKey": "nestedValue"})
	Equal(t, v.ObjectPtrNil, nil)
}

func TestNewJsonObjectFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    JsonObject
		wantErr bool
	}{
		{
			name:    "Valid JSON String",
			input:   `{"key":"value"}`,
			want:    JsonObject{Data: map[string]any{"key": "value"}},
			wantErr: false,
		},
		{
			name:    "Invalid JSON String",
			input:   `{"key":`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewJsonObjectFromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJsonObjectFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got.Data, tt.want.Data) {
				t.Errorf("NewJsonObjectFromString() got = %v, want %v", got.Data, tt.want.Data)
			}
		})
	}
}
