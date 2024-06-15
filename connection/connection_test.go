package connection

import (
	"reflect"
	"testing"

	_ "github.com/joho/godotenv/autoload"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/serror"
)

func TestConnectMinio(t *testing.T) {
	if helper.Env("MINIO_HOST") == "" {
		t.Log("TestConnectMinio skipped")
		return
	}
	tests := []struct {
		name    string
		wantErr serror.SError
	}{
		{
			name:    "00",
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got1 := ConnectMinio()
			if !reflect.DeepEqual(got1, tt.wantErr) {
				t.Errorf("ConnectMinio() got1 = %v, want %v", got1, tt.wantErr)
			}
		})
	}
}

func TestConnectAllPgsql(t *testing.T) {
	if //goland:noinspection GoBoolExpressions
	1 == 1 {
		return
	}
	if helper.Env("DB_NAME") == "" {
		t.Log("TestConnectAllPgsql test skipped")
		return
	}
	tests := []struct {
		name     string
		wantSerr serror.SError
	}{
		{
			wantSerr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotSerr := ConnectAllPgsql()
			t.Log(gotResult)
			if !reflect.DeepEqual(gotSerr, tt.wantSerr) {
				t.Errorf("ConnectAllPgsql() gotSerr = %v, want %v", gotSerr, tt.wantSerr)
			}
		})
	}
}

func TestConnectMysql(t *testing.T) {
	if //goland:noinspection GoBoolExpressions
	1 == 1 {
		return
	}
	_, serr := ConnectMysql("media_analysis", "", "", "localhost", "9306", "false")
	if serr != nil {
		t.Fatal(serr)
	}
	t.Log("ok")
}
