package types

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/go-playground/form"
	. "gopkg.in/go-playground/assert.v1"
)

func TestNativeDate_MarshalJSON(t1 *testing.T) {
	type fields struct {
		time.Time
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			fields: fields{
				Time: time.Date(2269, 12, 2, 0, 0, 0, 0, time.UTC),
			},
			want:    []byte(`"2269-12-02"`),
			wantErr: false,
		},
		{
			fields: fields{
				Time: time.Date(2242, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			want:    []byte(`"2242-12-31"`),
			wantErr: false,
		},
		{
			fields: fields{
				Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			want:    []byte(`"0001-01-01"`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &NativeDate{
				Time: tt.fields.Time,
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

func TestNativeDateDecoder_RegisterCustomTypeFunc(t *testing.T) {
	type TestStruct struct {
		Date       NativeDate  `form:"date"`
		DatePtr    *NativeDate `form:"date_ptr"`
		DatePtrNil *NativeDate `form:"date_ptr_nil"`
	}

	d := form.NewDecoder()
	d.RegisterCustomTypeFunc((&NativeDate{}).RegisterCustomTypeFunc(), NativeDate{})

	var v TestStruct
	err := d.Decode(&v, url.Values{
		"date":     []string{"2024-01-13"},
		"date_ptr": []string{"2030-12-12"},
	})

	Equal(t, err, nil)
	Equal(t, v.Date, NativeDate{Time: time.Date(2024, 1, 13, 0, 0, 0, 0, time.UTC)})
	Equal(t, v.DatePtr, &NativeDate{Time: time.Date(2030, 12, 12, 0, 0, 0, 0, time.UTC)})
	Equal(t, v.DatePtrNil, nil)
}

func TestNativeDate_UnmarshalJSON(t1 *testing.T) {
	type fields struct {
		Time []byte
	}
	tests := []struct {
		name    string
		fields  fields
		want    time.Time
		wantErr bool
	}{
		{
			fields: fields{
				Time: []byte(`"2020-02-02"`),
			},
			want:    time.Date(2020, 2, 2, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &NativeDate{}
			if err := json.Unmarshal(tt.fields.Time, &t); (err != nil) != tt.wantErr {
				t1.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got := t.Time
			if !reflect.DeepEqual(got, tt.want) {
				t1.Errorf("UnmarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNativeDate_Scan(t1 *testing.T) {
	type fields struct {
		Time time.Time
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
			name:   "date",
			fields: fields{Time: time.Date(2269, 12, 2, 0, 0, 0, 0, time.UTC)},
			args: args{
				time.Date(2269, 12, 2, 0, 0, 0, 0, time.UTC),
			},
			wantErr: false,
		},
		{
			name:   "bytes",
			fields: fields{Time: time.Date(2269, 12, 2, 0, 0, 0, 0, time.UTC)},
			args: args{
				[]byte(`"2269-12-02"`),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &NativeDate{
				Time: tt.fields.Time,
			}
			if err := t.Scan(tt.args.src); (err != nil) != tt.wantErr {
				t1.Errorf("Scan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(t.Time, tt.fields.Time) {
				t1.Errorf("Scan() got = %v, want %v", t.Time, tt.fields.Time)
			}
		})
	}
}

func TestNewNativeDateFromString(t *testing.T) {
	type args struct {
		dt string
	}
	tests := []struct {
		name    string
		args    args
		wantRes NativeDate
		wantErr bool
	}{
		{
			args:    args{`2023-05-13`},
			wantRes: NewNativeDate(time.Date(2023, 5, 13, 0, 0, 0, 0, time.UTC)),
			wantErr: false,
		},
		{
			args:    args{`"2020-12-22"`},
			wantRes: NewNativeDate(time.Date(2020, 12, 22, 0, 0, 0, 0, time.UTC)),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := NewNativeDateFromString(tt.args.dt)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewNativeDateFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("NewNativeDateFromString() gotRes = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
