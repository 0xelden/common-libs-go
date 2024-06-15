package shared

import (
	"testing"
	"time"

	"github.com/0xelden/common-libs-go/helper"
)

func TestDtvalueTag(t *testing.T) {
	type Args struct {
		A time.Time `validate:"dtvalue"`
	}
	tests := []struct {
		name    string
		args    Args
		wantErr bool
	}{
		{
			name: "00. valid date",
			args: Args{
				A: time.Now(),
			},
		},
		{
			name: "01. invalid date",
			args: Args{
				A: time.Time{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate.Struct(&tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestDtvalueTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDtvalueTagPtr(t *testing.T) {
	type Args struct {
		A *time.Time `validate:"dtvalue"`
	}
	tests := []struct {
		name    string
		args    Args
		wantErr bool
	}{
		{
			name: "00. valid date",
			args: Args{
				A: helper.Ptr(time.Now()),
			},
		},
		{
			name: "01. invalid date",
			args: Args{
				A: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate.Struct(&tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestDtvalueTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
