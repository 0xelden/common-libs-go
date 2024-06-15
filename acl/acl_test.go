package acl

import (
	"context"
	"testing"

	"github.com/0xelden/common-libs-go/shared"
)

func TestGetAccessLevel(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name string
		args args
		want Level
	}{
		{
			name: "00",
			args: args{
				ctx: context.WithValue(context.Background(), shared.X_Slug, "a/b/c"),
			},
			want: Local,
		},
		{
			name: "01",
			args: args{
				ctx: context.WithValue(context.Background(), shared.X_Slug, "a/b/gl-foo"),
			},
			want: Global,
		},
		{
			name: "02",
			args: args{
				ctx: context.WithValue(context.Background(), shared.X_Slug, "a/b/hd-foo"),
			},
			want: Holding,
		},
		{
			name: "03",
			args: args{
				ctx: context.Background(),
			},
			want: Local,
		},
		{
			name: "04",
			args: args{
				ctx: context.WithValue(context.Background(), shared.X_Slug, "a/b/hd-"),
			},
			want: Local,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAccessLevel(tt.args.ctx); got != tt.want {
				t.Errorf("GetAccessLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
