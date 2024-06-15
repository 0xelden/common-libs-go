package general

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/joho/godotenv/autoload"
	"github.com/0xelden/common-libs-go/connection"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	helper2 "github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/models"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/shared"
)

func Test_generalRepo_IncrementFields(t *testing.T) {
	if 1 == 1 {
		// TODO: fix this test
		return
	}

	if helper.Env(shared.DBName) == "" || helper.Env(shared.DBPort) == "" {
		t.Log("TEST SKIPPED: Test_generalRepo_IncrementFields")
		return
	}

	PG, serr := connection.ConnectPG()
	if serr != nil {
		t.Fatal(serr)
	}

	trx := db.NewTrxBuilder(PG)
	tx, err := trx.NewTransaction(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer tx.Rollback()

	type fields struct {
		db db.Driver
	}
	type args struct {
		ctx   context.Context
		param models.FieldParam
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   serror.SError
	}{
		{
			name:   "valid increment",
			fields: fields{PG},
			args: args{
				context.WithValue(context.Background(), shared.X_UserId, "c55f725f-cce9-44eb-be92-b8444b8bfded"),
				models.FieldParam{
					Table: "inventory.inv_outbound_item",
					Id:    "89b8ce55-da5d-4368-bbd7-0d648d3c7324",
					Field: "base_qty",
					Value: 12,
				},
			},
			want: nil,
		},
		{
			name:   "valid increment with slice of ids",
			fields: fields{PG},
			args: args{
				context.WithValue(context.Background(), shared.X_UserId, "c55f725f-cce9-44eb-be92-b8444b8bfded"),
				models.FieldParam{
					Table: "inventory.inv_outbound_item",
					Id:    []string{"89b8ce55-da5d-4368-bbd7-0d648d3c7324"},
					Field: "base_qty",
					Value: 12,
				},
			},
			want: nil,
		},
		{
			name:   "error on not found",
			fields: fields{PG},
			args: args{
				context.WithValue(context.Background(), shared.X_UserId, "c55f725f-cce9-44eb-be92-b8444b8bfded"),
				models.FieldParam{
					Table: "inventory.inv_outbound_item",
					Id:    "95592AD1-2736-42DB-8F26-8DECCBD7A66D",
					Field: "base_qty",
					Value: 12,
				},
			},
			want: serror.NewFromError(ErrGeneralRepoNotFound),
		},
		{
			name:   "error on invalid id",
			fields: fields{PG},
			args: args{
				context.WithValue(context.Background(), shared.X_UserId, "c55f725f-cce9-44eb-be92-b8444b8bfded"),
				models.FieldParam{
					Table: "inventory.inv_outbound_item",
					Id:    12,
					Field: "base_qty",
					Value: 12,
				},
			},
			want: serror.NewFromError(ErrGeneralRepoInvalidId),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := generalRepo{
				db:   tt.fields.db,
				pfmt: sq.Dollar,
			}
			got := g.IncrementFields(tt.args.ctx, tt.args.param)
			if (tt.want == nil && !reflect.DeepEqual(got, tt.want)) || (tt.want != nil && !tt.want.IsEqual(got)) {
				t.Errorf("IncrementFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generalRepo_DecrementFields(t *testing.T) {
	if 1 == 1 {
		// TODO: fix this test
		return
	}

	if helper.Env(shared.DBName) == "" || helper.Env(shared.DBPort) == "" {
		t.Log("TEST SKIPPED: Test_generalRepo_DecrementFields")
		return
	}

	PG, serr := connection.ConnectPG()
	if serr != nil {
		t.Fatal(serr)
	}

	trx := db.NewTrxBuilder(PG)
	tx, err := trx.NewTransaction(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer tx.Rollback()

	type fields struct {
		db db.Driver
	}
	type args struct {
		ctx   context.Context
		param models.FieldParam
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   serror.SError
	}{
		{
			name:   "valid decrement",
			fields: fields{PG},
			args: args{
				context.WithValue(context.Background(), shared.X_UserId, "c55f725f-cce9-44eb-be92-b8444b8bfded"),
				models.FieldParam{
					Table: "inventory.inv_outbound_item",
					Id:    "89b8ce55-da5d-4368-bbd7-0d648d3c7324",
					Field: "base_qty",
					Value: 12,
				},
			},
			want: nil,
		},
		{
			name:   "valid decrement with slice of ids",
			fields: fields{PG},
			args: args{
				context.WithValue(context.Background(), shared.X_UserId, "c55f725f-cce9-44eb-be92-b8444b8bfded"),
				models.FieldParam{
					Table: "inventory.inv_outbound_item",
					Id:    []string{"89b8ce55-da5d-4368-bbd7-0d648d3c7324"},
					Field: "base_qty",
					Value: 12,
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := generalRepo{
				db:   tt.fields.db,
				pfmt: sq.Dollar,
			}
			got := g.DecrementFields(tt.args.ctx, tt.args.param)
			if (tt.want == nil && !reflect.DeepEqual(got, tt.want)) || (tt.want != nil && !tt.want.IsEqual(got)) {
				t.Errorf("DecrementFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generalRepo_SelectScanRow(t *testing.T) {
	if 1 == 1 {
		// TODO: fix this test
		return
	}
	if helper.Env(shared.DBName) == "" || helper.Env(shared.DBPort) == "" {
		t.Log("test skipped: Test_generalRepo_SelectScanRow")
		return
	}
	pg, err := connection.ConnectPG()
	if err != nil {
		t.Fatal(err)
	}
	type User struct {
		Username string `db:"username"`
	}
	type fields struct {
		db    db.Driver
		table string
	}
	type args struct {
		ctx    context.Context
		result interface{}
		stmt   sq.SelectBuilder
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "00",
			fields: fields{db: pg},
			args: args{
				ctx:    context.Background(),
				result: User{Username: "olein_admin"},
				stmt:   sq.Select("username").From("usm.usm_user").Where(sq.Eq{"username": "olein_admin"}).Limit(1),
			},
			wantErr: false,
		},
		{
			name:   "01. optional from clause",
			fields: fields{db: pg, table: "usm.usm_user"},
			args: args{
				ctx:    context.Background(),
				result: User{Username: "olein_admin"},
				stmt:   sq.Select("username").Where(sq.Eq{"username": "olein_admin"}).Limit(1),
			},
			wantErr: false,
		},
		{
			name:   "02. default row limit to 1",
			fields: fields{db: pg, table: "usm.usm_user"},
			args: args{
				ctx:    context.Background(),
				result: User{Username: "admin"},
				stmt:   sq.Select("username").Where(sq.Like{"username": "%admin"}),
			},
			wantErr: false,
		},
		{
			name:   "03. error on not found",
			fields: fields{db: pg, table: "usm.usm_user"},
			args: args{
				ctx:    context.Background(),
				result: User{},
				stmt:   sq.Select("username").Where(sq.Eq{"username": "0lsdklsdlk"}),
			},
			wantErr: true,
		},
		{
			name:   "04. valid SetFromClause override",
			fields: fields{db: pg, table: "usm.usm_user_ksdksd"},
			args: args{
				ctx:    context.Background(),
				result: User{Username: "admin"},
				stmt:   sq.Select("username").Where(sq.Eq{"username": "admin"}),
			},
			wantErr: false,
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGeneralRepo(tt.fields.db, tt.fields.table)
			var got User
			tt.args.stmt = tt.args.stmt.From("usm.usm_user")
			if err := g.SelectScanRow(tt.args.ctx, &got, tt.args.stmt); (err != nil) != tt.wantErr {
				t.Errorf("SelectScanRow() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.args.result) {
				t.Errorf("SelectScanRow() got = %v, want %v", got, tt.args.result)
			}
		})
	}
}

func Test_generalRepo_SelectScanRows(t *testing.T) {
	if 1 == 1 {
		// TODO: fix this test
		return
	}
	if helper.Env(shared.DBName) == "" || helper.Env(shared.DBPort) == "" {
		t.Log("test skipped: Test_generalRepo_SelectScanRows")
		return
	}
	pg, err := connection.ConnectPG()
	if err != nil {
		t.Fatal(err)
	}
	type User struct {
		Username string `db:"username"`
	}
	type fields struct {
		db    db.Driver
		table string
	}
	type args struct {
		ctx    context.Context
		result interface{}
		stmt   sq.SelectBuilder
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "00",
			fields: fields{db: pg},
			args: args{
				ctx:    context.Background(),
				result: []User{{Username: "admin"}, {Username: "olein_admin"}},
				stmt:   sq.Select("username").From("usm.usm_user").Where(sq.Like{"username": "%admin"}),
			},
			wantErr: false,
		},
		{
			name:   "01. optional from clause",
			fields: fields{db: pg, table: "usm.usm_user"},
			args: args{
				ctx:    context.Background(),
				result: []User{{Username: "admin"}, {Username: "olein_admin"}},
				stmt:   sq.Select("username").Where(sq.Like{"username": "%admin"}),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGeneralRepo(tt.fields.db, tt.fields.table)
			var got []User
			if err := g.SelectScanRows(tt.args.ctx, &got, tt.args.stmt); (err != nil) != tt.wantErr {
				t.Errorf("SelectScanRows() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.args.result) {
				t.Errorf("SelectScanRows() got = %v, want %v", got, tt.args.result)
			}
		})
	}
}

func Test_generalRepo_InsertScanRow(t *testing.T) {
	type fields struct {
		db    db.Driver
		table *string
	}

	sqlite, serr := connection.ConnectSqliteInmemory()
	if serr != nil {
		t.Error(serr)
		return
	}

	//goland:noinspection GoUnhandledErrorResult
	defer sqlite.Close()

	type Foo struct {
		Id int `sqlite:"id"`
	}

	type Bar struct {
		Id string `sqlite:"id"`
	}

	type args struct {
		ctx    context.Context
		result interface{}
		stmt   sq.InsertBuilder
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   serror.SError
		ddl    string
	}{
		{
			name:   "00",
			fields: fields{sqlite, helper2.Ptr("foo")},
			args: args{
				ctx:    context.Background(),
				result: Foo{Id: 100},
				stmt: sq.Insert("foo").SetMap(map[string]any{
					"id": 100,
				}),
			},
			want: nil,
			ddl: `CREATE TABLE IF NOT EXISTS foo (
				id INTEGER
			)`,
		},
		{
			name:   "01",
			fields: fields{sqlite, helper2.Ptr("bar")},
			args: args{
				ctx:    context.Background(),
				result: Bar{Id: "9B6DCFA7-27FB-423E-945E-AE5A9217FEA1"},
				stmt: sq.Insert("bar").SetMap(map[string]any{
					"id": "9B6DCFA7-27FB-423E-945E-AE5A9217FEA1",
				}),
			},
			want: nil,
			ddl: `CREATE TABLE IF NOT EXISTS bar (
				id TEXT
			)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sqlite.Exec(tt.ddl)
			if err != nil {
				t.Error(serr)
				return
			}
			g := generalRepo{
				db:    tt.fields.db,
				table: tt.fields.table,
				pfmt:  sq.Dollar,
			}
			if got := g.InsertScanRow(tt.args.ctx, &tt.args.result, tt.args.stmt); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InsertScanRow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generalRepo_CountRows(t *testing.T) {
	sqlite, serr := connection.ConnectSqliteInmemory()
	if serr != nil {
		t.Fatal(serr)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer sqlite.Close()

	type fields struct {
		db    db.Driver
		table *string
	}
	type args struct {
		ctx  context.Context
		stmt sq.SelectBuilder
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantTotal int64
		wantSerr  serror.SError
		insert    bool
	}{
		{
			name: "00. use alias",
			fields: fields{
				db:    sqlite,
				table: helper2.Ptr("counter_0"),
			},
			args: args{
				ctx:  context.Background(),
				stmt: sq.Select("count(*) as total"),
			},
			wantTotal: 100,
			wantSerr:  nil,
			insert:    true,
		},
		{
			name: "01. no alias",
			fields: fields{
				db:    sqlite,
				table: helper2.Ptr("counter_1"),
			},
			args: args{
				ctx:  context.Background(),
				stmt: sq.Select("count(*)"),
			},
			wantTotal: 42,
			wantSerr:  nil,
			insert:    true,
		},
		{
			name: "02. use filter",
			fields: fields{
				db:    sqlite,
				table: helper2.Ptr("counter_1"),
			},
			args: args{
				ctx:  context.Background(),
				stmt: sq.Select("count(*)").Where(sq.GtOrEq{"id": 40}),
			},
			wantTotal: 2,
			wantSerr:  nil,
			insert:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.insert {
				_, err := sqlite.Exec(fmt.Sprintf(`create table %s (id int primary key )`, *tt.fields.table))
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < int(tt.wantTotal); i++ {
					_, err := sqlite.Exec(fmt.Sprintf(`insert into %s(id) values ($1)`, *tt.fields.table), i)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			g := generalRepo{
				db:    tt.fields.db,
				table: tt.fields.table,
				pfmt:  sq.Dollar,
			}
			gotTotal, gotSerr := g.CountRows(tt.args.ctx, tt.args.stmt)
			if gotTotal != tt.wantTotal {
				t.Errorf("CountRows() gotTotal = %v, want %v", gotTotal, tt.wantTotal)
			}
			if !reflect.DeepEqual(gotSerr, tt.wantSerr) {
				t.Errorf("CountRows() gotSerr = %v, want %v", gotSerr, tt.wantSerr)
			}
		})
	}
}
