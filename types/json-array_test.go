package types_test

import (
	"context"
	"net/url"
	"reflect"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-playground/form"
	"github.com/0xelden/common-libs-go/connection"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/service/repository/general"
	"github.com/0xelden/common-libs-go/types"
	. "gopkg.in/go-playground/assert.v1"
)

func TestScanJsonArray(t *testing.T) {
	if //goland:noinspection GoBoolExpressions
	1 == 1 {
		return
	}

	pg, serr := connection.ConnectPG()
	if serr != nil {
		t.Fatal(serr)
	}
	ctx := context.Background()
	tx, err := db.NewTrxBuilder(pg).NewTransaction(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tx == nil {
		t.Fatal("nil transaction")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `CREATE TABLE public.model_dto (
			id UUID PRIMARY KEY,
			data JSON,
			status INT,
			created_by VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE,
			updated_by VARCHAR(255),
			updated_at TIMESTAMP WITH TIME ZONE
		);`)
	if err != nil {
		t.Fatal(err)
	}

	type ModelDto struct {
		Id        string           `db:"id" json:"id" form:"id" validate:"omitempty,uuid"`
		Data      *types.JsonArray `db:"data" json:"data" form:"data"`
		Status    *int             `db:"status" json:"status" form:"status"`
		CreatedBy *string          `db:"created_by" json:"created_by" form:"created_by"`
		CreatedAt *time.Time       `db:"created_at" json:"created_at" form:"created_at"`
		UpdatedBy *string          `db:"updated_by" json:"updated_by" form:"updated_by"`
		UpdatedAt *time.Time       `db:"updated_at" json:"updated_at" form:"updated_at"`
	}

	gen := general.NewGeneralRepo(tx, "public.model_dto")
	dto := general.NewDTO[ModelDto]("public.model_dto", gen)
	param := []ModelDto{
		{
			Id: "9ab6f85b-0092-4b88-9869-0cef5fd26bfd",
			Data: helper.Ptr(types.NewJsonArray([]any{
				1,
				"Egestas a purus nec posuere interdum mauris vitae.",
				[]any{1, true, nil},
				nil,
				time.Now(),
			})),
		},
		{
			Id:   "fb6c95e4-6359-4aae-8481-b8f78e651006",
			Data: nil,
		},
	}
	rows, serr := dto.InsertRows(ctx, param, nil)
	if serr != nil {
		t.Fatal(serr)
	}
	t.Log(rows)

	res, serr := dto.SelectRows(ctx, sq.Select("*").Where(sq.Eq{
		"id": []string{"9ab6f85b-0092-4b88-9869-0cef5fd26bfd", "fb6c95e4-6359-4aae-8481-b8f78e651006"},
	}))
	if serr != nil {
		t.Fatal(serr)
	}

	t.Log(res)
}

func TestJsonArray_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		fields  []any
		want    []byte
		wantErr bool
	}{
		{
			name:    "Simple Array",
			fields:  []any{"value"},
			want:    []byte(`["value"]`),
			wantErr: false,
		},
		{
			name:    "Nested Array",
			fields:  []any{"value", []any{"nestedValue"}},
			want:    []byte(`["value",["nestedValue"]]`),
			wantErr: false,
		},
		{
			name:    "Empty Array",
			fields:  []any{},
			want:    []byte(`[]`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := types.JsonArray{Data: tt.fields}
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

func TestJsonArray_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []any
		wantErr bool
	}{
		{
			name:    "Valid Array",
			input:   []byte(`["value"]`),
			want:    []any{"value"},
			wantErr: false,
		},
		{
			name:    "Nested Array",
			input:   []byte(`["value",["nestedValue"]]`),
			want:    []any{"value", []any{"nestedValue"}},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   []byte(`["value",`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &types.JsonArray{}
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

func TestJsonArray_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    []any
		wantErr bool
	}{
		{
			name:    "Valid JSON Bytes",
			input:   []byte(`["value"]`),
			want:    []any{"value"},
			wantErr: false,
		},
		{
			name:    "Invalid JSON Bytes",
			input:   []byte(`["value",`),
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
			obj := &types.JsonArray{}
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

func TestJsonArrayDecoder_RegisterCustomTypeFunc(t *testing.T) {
	type TestStruct struct {
		Array       types.JsonArray  `form:"array"`
		ArrayPtr    *types.JsonArray `form:"array_ptr"`
		ArrayPtrNil *types.JsonArray `form:"array_ptr_nil"`
	}

	d := form.NewDecoder()
	d.RegisterCustomTypeFunc((&types.JsonArray{}).RegisterCustomTypeFunc(), types.JsonArray{})

	var v TestStruct
	err := d.Decode(&v, url.Values{
		"array":     []string{`["value", 1, false, true, null]`},
		"array_ptr": []string{`["nestedValue"]`},
	})

	Equal(t, err, nil)
	Equal(t, v.Array.Data, []any{"value", 1.0, false, true, nil})
	Equal(t, v.ArrayPtr.Data, []any{"nestedValue"})
	Equal(t, v.ArrayPtrNil, nil)
}

func TestNewJsonArrayFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    types.JsonArray
		wantErr bool
	}{
		{
			name:    "Valid JSON String",
			input:   `["value", 10.0]`,
			want:    types.JsonArray{Data: []any{"value", 10.0}},
			wantErr: false,
		},
		{
			name:    "Invalid JSON String",
			input:   `["value",`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := types.NewJsonArrayFromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJsonArrayFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got.Data, tt.want.Data) {
				t.Errorf("NewJsonArrayFromString() got = %v, want %v", got.Data, tt.want.Data)
			}
		})
	}
}
