package types

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/go-playground/form"
	"github.com/jackc/pgtype"
	. "gopkg.in/go-playground/assert.v1"
)

func TestNativeTime_MarshalJSON(t1 *testing.T) {
	type fields struct {
		pgtype.Time
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			fields: fields{
				Time: pgtype.Time{Microseconds: 43_932_000_000, Status: 2},
			},
			want:    []byte(`"12:12:12"`),
			wantErr: false,
		},
		{
			fields: fields{
				Time: pgtype.Time{Microseconds: 1_000_000, Status: 2},
			},
			want:    []byte(`"00:00:01"`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &NativeTime{
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

func TestNativeTimeDecoder_RegisterCustomTypeFunc(t *testing.T) {
	type TestStruct struct {
		Time       NativeTime  `form:"time"`
		TimePtr    *NativeTime `form:"time_ptr"`
		TimePtrNil *NativeTime `form:"time_ptr_nil"`
	}

	d := form.NewDecoder()
	d.RegisterCustomTypeFunc((&NativeTime{}).RegisterCustomTypeFunc(), NativeTime{})

	var v TestStruct
	err := d.Decode(&v, url.Values{
		"time":     []string{"12:12:12"},
		"time_ptr": []string{"00:00:01"},
	})

	Equal(t, err, nil)
	Equal(t, v.Time, NativeTime{Time: pgtype.Time{Microseconds: 43_932_000_000, Status: 2}})
	Equal(t, v.TimePtr, NativeTime{Time: pgtype.Time{Microseconds: 1000000, Status: 2}})
	Equal(t, v.TimePtrNil, nil)
}

func TestNativeTime_UnmarshalJSON(t1 *testing.T) {
	type fields struct {
		Time []byte
	}
	tests := []struct {
		name    string
		fields  fields
		want    pgtype.Time
		wantErr bool
	}{
		{
			fields: fields{
				Time: []byte(`"12:12:12"`),
			},
			want:    pgtype.Time{Microseconds: 43_932_000_000, Status: 2},
			wantErr: false,
		},
		{
			fields: fields{
				Time: []byte(`"00:00:00"`),
			},
			want:    pgtype.Time{Microseconds: 0, Status: 2},
			wantErr: false,
		},
		{
			fields: fields{
				Time: []byte(`"00:00:01"`),
			},
			want:    pgtype.Time{Microseconds: 1_000_000, Status: 2},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &NativeTime{}
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

func TestNativeTime_Scan(t1 *testing.T) {
	type fields struct {
		Time pgtype.Time
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
			name:   "time.Time",
			fields: fields{Time: pgtype.Time{Microseconds: 43_932_000_000, Status: 2}},
			args: args{
				time.Date(0, 0, 0, 12, 12, 12, 0, time.UTC),
			},
			wantErr: false,
		},
		{
			name:   "bytes",
			fields: fields{Time: pgtype.Time{Microseconds: 43_932_000_000, Status: 2}},
			args: args{
				[]byte(`12:12:12`),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &NativeTime{
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

func TestNewNativeTimeFromString(t *testing.T) {
	type args struct {
		Time string
	}
	tests := []struct {
		name    string
		args    args
		wantRes NativeTime
		wantErr bool
	}{
		// Valid Cases
		{
			name: "Valid Time - 10:10:00",
			args: args{Time: "10:10:00"},
			wantRes: NativeTime{
				Time: pgtype.Time{
					Microseconds: 36_600_000_000,
					Status:       pgtype.Present,
				},
			},
			wantErr: false,
		},
		{
			name: "Valid Time - Midnight (00:00:00)",
			args: args{Time: "00:00:00"},
			wantRes: NativeTime{
				Time: pgtype.Time{
					Microseconds: 0,
					Status:       pgtype.Present,
				},
			},
			wantErr: false,
		},
		{
			name: "Valid Time - One second before midnight (23:59:59)",
			args: args{Time: "23:59:59"},
			wantRes: NativeTime{
				Time: pgtype.Time{
					Microseconds: 86_399_000_000, // (23*60*60 + 59*60 + 59) * 1,000,000
					Status:       pgtype.Present,
				},
			},
			wantErr: false,
		},
		{
			name: "Valid Time - 24:00:00 (Special Case)",
			args: args{Time: "24:00:00"},
			wantRes: NativeTime{
				Time: pgtype.Time{
					Microseconds: 86_400_000_000, // (24*60*60) * 1,000,000
					Status:       pgtype.Present,
				},
			},
			wantErr: false,
		},
		{
			name: "Valid Time - Millisecond precision (10:10:10.123456)",
			args: args{Time: "10:10:10.123456"},
			wantRes: NativeTime{
				Time: pgtype.Time{
					Microseconds: 36_610_123_456, // 10:10:10.123456 = (10*3600 + 10*60 + 10) * 1,000,000 + 123456
					Status:       pgtype.Present,
				},
			},
			wantErr: false,
		},

		// Invalid Cases
		{
			name:    "Invalid Time - Negative hours (-01:00:00)",
			args:    args{Time: "-01:00:00"},
			wantErr: true,
		},
		{
			name:    "Invalid Time - Non-numeric (hello:world)",
			args:    args{Time: "hello:world"},
			wantErr: true,
		},
		{
			name:    "Invalid Time - Missing colon (121212)",
			args:    args{Time: "121212"},
			wantErr: true,
		},
		{
			name:    "Invalid Time - Empty string",
			args:    args{Time: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := NewNativeTimeFromString(tt.args.Time)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewNativeTimeFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("NewNativeTimeFromString() gotRes = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
