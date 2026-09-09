package auth

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/0xelden/common-libs-go/shared"
	"github.com/gin-gonic/gin"
)

// exampleSessionJSON is a single-tenant session payload: roles is a list at the
// root and there is no "company" object at all, unlike the legacy
// uzzeet-usm-service shape covered in auth_test.go. The user carries two roles
// so the list path is what gets exercised, not a one-element special case.
const exampleSessionJSON = `{
	"user_id": "3ac71f28-9d05-4e6b-8c31-0b7f5a2e9d14",
	"username": "dealer.editor",
	"email": "dealer.editor@example.co.id",
	"name": "Dealer Content Editor",
	"profile_image": "",
	"phone_number": "",
	"key_data": "ses-data-3ac71f28-9d05-4e6b-8c31-0b7f5a2e9d14",
	"permission_type": "role_access",
	"menus": [
		{
			"id": "6ce1f34d-5c40-46af-9724-1f65d4cc0af7",
			"parent": null,
			"slug": "/cms",
			"name": "CMS",
			"type": "menu-parent",
			"children": [
				{"id": "000f1e3f-7fa1-467d-8845-82686605645f", "slug": "/dealer", "name": "Dealers", "type": "menu", "children": []},
				{"id": "6aa52c4b-3f6b-4f74-929f-4c7dcd5926cf", "slug": "/product", "name": "Products", "type": "menu", "children": []}
			]
		}
	],
	"roles": [
		{"id": "b41d8e07-2f6a-4c19-9e83-71ad0c5b3f62", "name": "Dealer Admin", "level": 1},
		{"id": "e97c3a15-5b28-4d70-86f4-2c1e9b0a7d38", "name": "Product Editor", "level": 2}
	]
}`

func int64Ptr(v int64) *int64 { return &v }

func TestSessionEnvelopeParsesRoles(t *testing.T) {
	var session SessionEnvelope
	if err := json.Unmarshal([]byte(exampleSessionJSON), &session); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(session.Roles) != 2 {
		t.Fatalf("Roles length = %d, want 2", len(session.Roles))
	}

	want := []SessionRole{
		{Id: "b41d8e07-2f6a-4c19-9e83-71ad0c5b3f62", Name: "Dealer Admin", Level: int64Ptr(1)},
		{Id: "e97c3a15-5b28-4d70-86f4-2c1e9b0a7d38", Name: "Product Editor", Level: int64Ptr(2)},
	}
	for i, w := range want {
		got := session.Roles[i]
		if got.Id != w.Id {
			t.Errorf("Roles[%d].Id = %q, want %q", i, got.Id, w.Id)
		}
		if got.Name != w.Name {
			t.Errorf("Roles[%d].Name = %q, want %q", i, got.Name, w.Name)
		}
		if got.Level == nil || *got.Level != *w.Level {
			t.Errorf("Roles[%d].Level = %v, want %d", i, got.Level, *w.Level)
		}
	}
}

// Order matters: the ids are joined into one context value and compared
// positionally by anything downstream, so parsing must not reorder them.
func TestSessionEnvelopeRoleIdsFromPayloadKeepsOrder(t *testing.T) {
	var session SessionEnvelope
	if err := json.Unmarshal([]byte(exampleSessionJSON), &session); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := JoinRoleIds(session.RoleIds())
	want := "b41d8e07-2f6a-4c19-9e83-71ad0c5b3f62,e97c3a15-5b28-4d70-86f4-2c1e9b0a7d38"
	if got != want {
		t.Errorf("JoinRoleIds(RoleIds()) = %q, want %q", got, want)
	}
}

// A payload with no company must still parse — that field is only kept for
// older sessions and must not become a requirement.
func TestSessionEnvelopeCompanyOptionalAlongsideRoles(t *testing.T) {
	var session SessionEnvelope
	if err := json.Unmarshal([]byte(exampleSessionJSON), &session); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if session.Company.Id != "" {
		t.Errorf("Company.Id = %q, want empty for a single-tenant payload", session.Company.Id)
	}
}

func TestSessionEnvelopeRoleIds(t *testing.T) {
	cases := map[string]struct {
		roles []SessionRole
		want  []string
	}{
		"single role":  {[]SessionRole{{Id: "a"}}, []string{"a"}},
		"many roles":   {[]SessionRole{{Id: "a"}, {Id: "b"}, {Id: "c"}}, []string{"a", "b", "c"}},
		"no roles":     {nil, nil},
		"empty slice":  {[]SessionRole{}, nil},
		"skips blanks": {[]SessionRole{{Id: "a"}, {Id: ""}, {Id: "b"}}, []string{"a", "b"}},
		// every id blank must collapse to nil, not to a slice of empties that
		// would later join into a stray separator.
		"all blank": {[]SessionRole{{Id: ""}, {Id: ""}}, nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := SessionEnvelope{Roles: tc.roles}.RoleIds()
			if len(got) != len(tc.want) {
				t.Fatalf("RoleIds() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("RoleIds()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// The list travels as one string, so encoding and decoding have to agree.
func TestJoinRoleIdsRoundTrip(t *testing.T) {
	cases := map[string][]string{
		"none":   nil,
		"one":    {"b41d8e07-2f6a-4c19-9e83-71ad0c5b3f62"},
		"two":    {"b41d8e07-2f6a-4c19-9e83-71ad0c5b3f62", "e97c3a15-5b28-4d70-86f4-2c1e9b0a7d38"},
		"threes": {"a", "b", "c"},
	}

	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), shared.X_RoleIds, JoinRoleIds(ids)) //nolint:staticcheck // key type is fixed by the context plumbing

			got := RoleIdsFromContext(ctx)
			if len(got) != len(ids) {
				t.Fatalf("round trip = %v, want %v", got, ids)
			}
			for i := range got {
				if got[i] != ids[i] {
					t.Errorf("round trip[%d] = %q, want %q", i, got[i], ids[i])
				}
			}
		})
	}
}

func TestRoleIdsFromContext(t *testing.T) {
	cases := map[string]struct {
		set  bool
		raw  string
		want []string
	}{
		"unset":               {false, "", nil},
		"empty":               {true, "", nil},
		"single":              {true, "a", []string{"a"}},
		"many":                {true, "a,b,c", []string{"a", "b", "c"}},
		"tolerates spaces":    {true, "a, b ,c", []string{"a", "b", "c"}},
		"drops empty entries": {true, "a,,b", []string{"a", "b"}},
		"separator only":      {true, ",", nil},
		"trailing separator":  {true, "a,", []string{"a"}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if tc.set {
				ctx = context.WithValue(ctx, shared.X_RoleIds, tc.raw) //nolint:staticcheck // key type is fixed by the context plumbing
			}

			got := RoleIdsFromContext(ctx)
			if len(got) != len(tc.want) {
				t.Fatalf("RoleIdsFromContext(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// A handler holds a *gin.Context, which resolves string keys against its own
// Keys map — so the same accessor has to work before api.SetContext has
// derived a native context from it.
func TestRoleIdsFromGinContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set(shared.X_RoleIds, JoinRoleIds([]string{"a", "b"}))

	got := RoleIdsFromContext(c)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("RoleIdsFromContext(gin) = %v, want [a b]", got)
	}
}

func TestHasRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), shared.X_RoleIds, "a,b") //nolint:staticcheck // key type is fixed by the context plumbing

	cases := map[string]struct {
		roleId string
		want   bool
	}{
		"first":  {"a", true},
		"second": {"b", true},
		"absent": {"c", false},
		// an empty id must never match, or a caller passing an unset value
		// would be treated as holding a role.
		"empty never matches": {"", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := HasRole(ctx, tc.roleId); got != tc.want {
				t.Errorf("HasRole(%q) = %v, want %v", tc.roleId, got, tc.want)
			}
		})
	}
}

func TestHasRoleWithNoRoles(t *testing.T) {
	if HasRole(context.Background(), "a") {
		t.Error("HasRole() = true with no roles on the context, want false")
	}
}
