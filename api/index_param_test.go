package api

import "testing"

func TestIndexParam_GenerateFilterStmt(t *testing.T) {
	type fields struct {
		Page     int
		Size     int
		Filter   string
		Sort     []string
		SortStmt string
		Filters  []string
	}
	type args struct {
		useAndClause []bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name:   "",
			fields: fields{},
			args:   args{},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &IndexParam{
				Page:     tt.fields.Page,
				Size:     tt.fields.Size,
				Filter:   tt.fields.Filter,
				Sort:     tt.fields.Sort,
				SortStmt: tt.fields.SortStmt,
				Filters:  tt.fields.Filters,
			}
			if got := i.GenerateFilterStmt(tt.args.useAndClause...); got != tt.want {
				t.Errorf("GenerateFilterStmt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndexParam_parseFilter(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr bool
	}{
		{
			name: "00. valid",
			args: "id > 1",
			want: "id>1",
		},
		{
			name: "01. valid",
			args: "((a=1 || a=2) && (c=1))",
			want: "((a=1 OR a=2) AND (c=1))",
		},
		{
			name: "02. valid is null",
			args: "a = null",
			want: "a IS NULL",
		},
		{
			name: "03. valid is not null",
			args: "a != null",
			want: "a IS NOT NULL",
		},
		{
			name: "04. valid null and not null",
			args: "((a=1 || a!= NUll) && (c=null))",
			want: "((a=1 OR a IS NOT NULL) AND (c IS NULL))",
		},
		{`> 1`, `> 1`, "", true},
		{`a >`, `a >`, "", true},
		{`a > >`, `a > >`, "", true},
		{`a > %`, `a > %`, "", true},
		{`a ! 1`, `a ! 1`, "", true},
		{`a - 1`, `a - 1`, "", true},
		{`a + 1`, `a + 1`, "", true},
		{`1 - 1`, `1 - 1`, "", true},
		{`1 + 1`, `1 + 1`, "", true},
		{`> a 1`, `> a 1`, "", true},
		{`a || 1`, `a || 1`, "", true},
		{`a && 1`, `a && 1`, "", true},
		{`test > 1 &&`, `test > 1 &&`, "", true},
		{`|| test = 1`, `|| test = 1`, "", true},
		{`test = 1 && ||`, `test = 1 && ||`, "", true},
		{`test = 1 && a`, `test = 1 && a`, "", true},
		{`test = 1 && a`, `test = 1 && a`, "", true},
		{`test = 1 && "a"`, `test = 1 && "a"`, "", true},
		{`test = 1 a`, `test = 1 a`, "", true},
		{`test = 1 a`, `test = 1 a`, "", true},
		{`test = 1 "a"`, `test = 1 "a"`, "", true},
		{`test = 1@test`, `test = 1@test`, "", true},
		{`test = .@test`, `test = .@test`, "", true},
		// mismatched text quotes
		{`test = "demo'`, `test = "demo'`, "", true},
		{`test = 'demo"`, `test = 'demo"`, "", true},
		{`test = 'demo'"`, `test = 'demo'"`, "", true},
		{`test = 'demo''`, `test = 'demo''`, "", true},
		{`test = "demo"'`, `test = "demo"'`, "", true},
		{`test = "demo""`, `test = "demo""`, "", true},
		{`test = ""demo""`, `test = ""demo""`, "", true},
		{`test = ''demo''`, `test = ''demo''`, "", true},
		{"test = `demo`", "test = `demo`", "", true},
		// valid simple expression and sign operators check
		{`1=12`, `1=12`, `1=12`, false},

		{`1='12'`, `1='12'`, `1='12'`, false},
		{`1='12"'`, `1='12"'`, `1='12"'`, false},

		{`   1    =    12    `, `1    =    12    `, `1=12`, false},
		{`"demo" != test`, `"demo" != test`, `'demo'!=test`, false},
		{`a~1`, `a~1`, `a ILIKE 1`, false},
		{`a !~ 1`, `a !~ 1`, `a!~1`, false},
		{`test>12`, `test>12`, `test>12`, false},
		{`test > 12`, `test > 12`, `test>12`, false},
		{`test >="test"`, `test >="test"`, `test>='test'`, false},
		{`test<@demo.test2`, `test<@demo.test2`, `test<@demo.test2`, false},
		{`1<="test"`, `1<="test"`, `1<='test'`, false},
		{`1<="te'st"`, `1<="te'st"`, `1<='te''st'`, false},
		{`demo='te\'st'`, `demo='te\'st'`, `demo='te''st'`, false},
		{`demo="te'st"`, `demo="te'st"`, `demo='te''st'`, false},
		{`demo="te\"st"`, `demo="te\"st"`, `demo='te"st'`, false},

		{``, `product_code::text = '%MANSET 5" X 4.5 MM X 400 MM'`, `product_code::text='%MANSET 5" X 4.5 MM X 400 MM'`, false},
		{``, `((product_code::text = '%MANSET 5\" X 4.5 MM X 400 MM'))`, `((product_code::text='%MANSET 5\" X 4.5 MM X 400 MM'))`, false},
		{``, `(product_code::text = '%MANSET 5\" X 4.5 MM X 400 MM')`, `(product_code::text='%MANSET 5\" X 4.5 MM X 400 MM')`, false},
		{``, `product_code::text != '%MANSET 5" X 4.5 MM X 400 MM'`, `product_code::text!='%MANSET 5" X 4.5 MM X 400 MM'`, false},
		{``, `product_code::text ~ '%MANSET 5" X 4.5 MM X 400 MM'`, `product_code::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM'`, false},
		{``, `(product_code::text ~ '%MANSET 5" X 4.5 MM X 400 MM' || product_code::text ~ '%MANSET 5" X 4.5 MM X 400 MM%' || product_code::text ~ 'MANSET 5" X 4.5 MM X 400 MM%') || (product_name::text ~ '%MANSET 5" X 4.5 MM X 400 MM' || product_name::text ~ '%MANSET 5" X 4.5 MM X 400 MM%' || product_name::text ~ 'MANSET 5" X 4.5 MM X 400 MM%') || (base_uom_name::text ~ '%MANSET 5" X 4.5 MM X 400 MM' || base_uom_name::text ~ '%MANSET 5" X 4.5 MM X 400 MM%' || base_uom_name::text ~ 'MANSET 5" X 4.5 MM X 400 MM%') || (category_name::text ~ '%MANSET 5" X 4.5 MM X 400 MM' || category_name::text ~ '%MANSET 5" X 4.5 MM X 400 MM%' || category_name::text ~ 'MANSET 5" X 4.5 MM X 400 MM%')`, `(product_code::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM' OR product_code::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM%' OR product_code::text ILIKE 'MANSET 5" X 4.5 MM X 400 MM%') OR (product_name::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM' OR product_name::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM%' OR product_name::text ILIKE 'MANSET 5" X 4.5 MM X 400 MM%') OR (base_uom_name::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM' OR base_uom_name::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM%' OR base_uom_name::text ILIKE 'MANSET 5" X 4.5 MM X 400 MM%') OR (category_name::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM' OR category_name::text ILIKE '%MANSET 5" X 4.5 MM X 400 MM%' OR category_name::text ILIKE 'MANSET 5" X 4.5 MM X 400 MM%')`, false},

		// invalid parenthesis
		{`a=1)`, `a=1)`, ``, true},
		{`((a=1)`, `((a=1)`, ``, true},
		{`{a=1}`, `{a=1}`, ``, true},
		{`[a=1]`, `[a=1]`, ``, true},
		{`((a=1 || a=2) && c=1))`, `((a=1 || a=2) && c=1))`, ``, true},
		// valid parenthesis
		{`(a=1)`, `(a=1)`, `(a=1)`, false},
		{`(a="test(")`, `(a="test(")`, `(a='test(')`, false},
		{`(a='test(\"')`, `(a='test(\"')`, `(a='test(\"')`, false},
		{`((a='test(\"'))`, `((a='test((\"'))`, `((a='test((\"'))`, false},
		{`((a~'test(\"'))`, `((a~'test((\"'))`, `((a ILIKE 'test((\"'))`, false},
		{`(a="test)")`, `(a="test)")`, `(a='test)')`, false},
		{`((a=1))`, `((a=1))`, `((a=1))`, false},
		{`a=1 || 2!=3`, `a=1 || 2!=3`, `a=1 OR 2!=3`, false},
		{`a=1 && 2!=3`, `a=1 && 2!=3`, `a=1 AND 2!=3`, false},
		{`a=1 && 2!=3 || "b"=a`, `a=1 && 2!=3 || "b"=a`, `a=1 AND 2!=3 OR 'b'=a`, false},
		{`(a=1 && 2!=3) || "b"=a`, `(a=1 && 2!=3) || "b"=a`, `(a=1 AND 2!=3) OR 'b'=a`, false},
		{`((a=1 || a=2) && (c=1))`, `((a=1 || a=2) && (c=1))`, `((a=1 OR a=2) AND (c=1))`, false},

		// valid like operator
		{`i ~ 'you%'`, `i ~ 'you%'`, `i ILIKE 'you%'`, false},

		// jsonb path (>= 3 dot-segments): alias.column.key[...]
		{`jsonb 01. text equality`, `result.content.name = 'Daisuke'`, `result.content->>'name'='Daisuke'`, false},
		{`jsonb 02. ilike`, `result.content.content ~ '%pangsa%'`, `result.content->>'content' ILIKE '%pangsa%'`, false},
		{`jsonb 03. nested keys`, `result.content.a.b = 'x'`, `result.content->'a'->>'b'='x'`, false},
		{`jsonb 04. is null`, `result.content.name = null`, `result.content->>'name' IS NULL`, false},
		{`jsonb 05. is not null`, `result.content.name != null`, `result.content->>'name' IS NOT NULL`, false},
		{`jsonb 06. numeric cast on number literal`, `result.content.price > 100`, `(result.content->>'price')::numeric>100`, false},
		{`jsonb 07. no cast on text literal`, `result.content.price > '100'`, `result.content->>'price'>'100'`, false},
		{`jsonb 08. combined with plain filter`, `status = 1 && result.content.name ~ 'dai%'`, `status=1 AND result.content->>'name' ILIKE 'dai%'`, false},
		// two segments stay plain alias.column (unchanged behavior)
		{`jsonb 09. two segments untouched`, `w.company_id = 'x'`, `w.company_id='x'`, false},
		// injection attempts via path segments are rejected, not emitted
		{`jsonb 10. cast in segment rejected`, `result.content.name::text = 'x'`, ``, true},
		{`jsonb 11. numeric-ish segment rejected`, `result.content.1a = 'x'`, ``, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &IndexParam{FilterMap: map[string]string{}}
			got, err := i.parseFilter(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseFilter() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndexParam_generateSortStmt_Jsonb(t *testing.T) {
	tests := []struct {
		name    string
		sort    []string
		want    string
		wantErr bool
	}{
		{"plain column", []string{"title:desc"}, `ORDER BY "title" DESC`, false},
		{"jsonb path", []string{"content.name:desc"}, `ORDER BY content->>'name' DESC`, false},
		{"jsonb nested", []string{"content.a.b:asc"}, `ORDER BY content->'a'->>'b' ASC`, false},
		{"mixed", []string{"content.name:asc", "published_at:desc"}, `ORDER BY content->>'name' ASC, "published_at" DESC`, false},
		{"invalid segment rejected", []string{"content.na'me:asc"}, ``, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &IndexParam{Sort: tt.sort}
			got, err := i.generateSortStmt()
			if (err != nil) != tt.wantErr {
				t.Errorf("generateSortStmt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("generateSortStmt() got = %v, want %v", got, tt.want)
			}
		})
	}
}
