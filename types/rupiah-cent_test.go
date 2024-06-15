package types

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"

	"github.com/go-playground/form"
	. "gopkg.in/go-playground/assert.v1"
)

func TestRupiahCent_MarshalJSON(t1 *testing.T) {
	type fields struct {
		int64 int64
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			fields: fields{
				int64: 426969,
			},
			want:    []byte(`426969`),
			wantErr: false,
		},
		{
			fields: fields{
				int64: 100426969,
			},
			want:    []byte(`100426969`),
			wantErr: false,
		},
		{
			fields: fields{
				int64: -100426969,
			},
			want:    []byte(`-100426969`),
			wantErr: false,
		},
		{
			fields: fields{
				int64: 0,
			},
			want:    []byte(`0`),
			wantErr: false,
		},
		{
			fields: fields{
				int64: 1,
			},
			want:    []byte(`1`),
			wantErr: false,
		},
		{
			fields: fields{
				int64: 100,
			},
			want:    []byte(`100`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &RupiahCent{
				Int64: tt.fields.int64,
			}
			got, err := t.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t1.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("MarshalJSON() got = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestDecoder_RegisterCustomTypeFunc(t *testing.T) {
	type TestStruct struct {
		Cent        RupiahCent  `form:"cent"`
		CentPtr     *RupiahCent `form:"cent_ptr"`
		CentPtrNil  *RupiahCent `form:"cent_ptr_nil"`
		FraqCent    RupiahCent  `form:"fraq_cent"`
		NegFraqCent RupiahCent  `form:"neg_fraq_cent"`
	}

	d := form.NewDecoder()
	d.RegisterCustomTypeFunc((&RupiahCent{}).RegisterCustomTypeFunc(), RupiahCent{})

	var v TestStruct
	err := d.Decode(&v, url.Values{
		"slice":         []string{"v1", "v2"},
		"cent":          []string{"426996"},
		"cent_ptr":      []string{"426996"},
		"fraq_cent":     []string{"123.696900001"},
		"neg_fraq_cent": []string{"-123.696900001"},
	})

	Equal(t, err, nil)
	Equal(t, v.Cent, RupiahCent{Int64: 42699600})
	Equal(t, v.CentPtr, &RupiahCent{Int64: 42699600})
	Equal(t, v.CentPtrNil, nil)
	Equal(t, v.FraqCent, RupiahCent{Int64: 12369})
	Equal(t, v.NegFraqCent, RupiahCent{Int64: -12369})
}

func TestRupiahCent_UnmarshalJSON(t1 *testing.T) {
	type fields struct {
		Int64 []byte
	}
	tests := []struct {
		name    string
		fields  fields
		want    int64
		wantErr bool
	}{
		{
			name: "00",
			fields: fields{
				Int64: []byte("426969"),
			},
			want:    42696900,
			wantErr: false,
		},
		{
			name: "01",
			fields: fields{
				Int64: []byte("1004269.69"),
			},
			want:    100426969,
			wantErr: false,
		},
		{
			name: "02",
			fields: fields{
				Int64: []byte("-1004269.69"),
			},
			want:    -100426969,
			wantErr: false,
		},
		{
			name: "03",
			fields: fields{
				Int64: []byte("0.000000001"),
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "04",
			fields: fields{
				Int64: []byte("0.01"),
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "05",
			fields: fields{
				Int64: []byte("1.00"),
			},
			want:    100,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &RupiahCent{}
			if err := json.Unmarshal(tt.fields.Int64, &t); (err != nil) != tt.wantErr {
				t1.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got := t.Int64
			if !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("UnmarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRupiahCent_NewRupiahCentFromFloat(t1 *testing.T) {
	type fields struct {
		Float64 float64
	}
	tests := []struct {
		name    string
		fields  fields
		want    int64
		wantErr bool
	}{
		{
			name: "00",
			fields: fields{
				Float64: 426969,
			},
			want:    42696900,
			wantErr: false,
		},
		{
			name: "01",
			fields: fields{
				Float64: 1004269.69,
			},
			want:    100426969,
			wantErr: false,
		},
		{
			name: "02",
			fields: fields{
				Float64: -1004269.69,
			},
			want:    -100426969,
			wantErr: false,
		},
		{
			name: "03",
			fields: fields{
				Float64: 0.000000001,
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "04",
			fields: fields{
				Float64: 0.01,
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "05",
			fields: fields{
				Float64: 1.00,
			},
			want:    100,
			wantErr: false,
		},
		{
			name: "06",
			fields: fields{
				Float64: 65446.494545,
			},
			want:    6544649,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := NewRupiahCentFromFloat(tt.fields.Float64)
			got := t.Int64
			if !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("UnmarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRupiahCent_Scan(t1 *testing.T) {
	type fields struct {
		Int64 int64
	}
	type args struct {
		src any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "int64",
			fields: fields{Int64: 1004269},
			args: args{
				int64(1004269),
			},
			wantErr: false,
		}, {
			name:   "int",
			fields: fields{Int64: 1004269},
			args: args{
				int(1004269),
			},
			wantErr: false,
		}, {
			name:   "int8",
			fields: fields{Int64: 100},
			args: args{
				int8(100),
			},
			wantErr: false,
		}, {
			name:   "int16",
			fields: fields{Int64: 42},
			args: args{
				int16(42),
			},
			wantErr: false,
		}, {
			name:   "int32",
			fields: fields{Int64: 1004269},
			args: args{
				int32(1004269),
			},
			wantErr: false,
		}, {
			name:   "float32",
			fields: fields{Int64: 1004269},
			args: args{
				float32(1004269),
			},
			wantErr: false,
		}, {
			name:   "float64",
			fields: fields{Int64: 1004269},
			args: args{
				float64(1004269),
			},
			wantErr: false,
		}, {
			name:   "[]uint8",
			fields: fields{Int64: 1004269},
			args: args{
				[]uint8("1004269"),
			},
			wantErr: false,
		}, {
			name:   "string",
			fields: fields{Int64: 1004269},
			args: args{
				string("1004269"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &RupiahCent{
				Int64: tt.fields.Int64,
			}
			if err := t.Scan(tt.args.src); (err != nil) != tt.wantErr {
				t1.Errorf("Scan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(t.Int64, tt.fields.Int64) {
				t1.Errorf("Scan() got = %v, want %v", t.Int64, tt.fields.Int64)
			}
		})
	}
}
