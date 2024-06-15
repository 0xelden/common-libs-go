package auth

import (
	"encoding/json"
	"testing"
)

// sample mirrors the shape uzzeet-usm-service actually writes to
// ses-data-<user_id>: AccessTokenClaims fields flattened at the root, plus a
// nested "company" object carrying its own apps/portals/menu tree. Only
// company.id matters to SessionEnvelope; everything else must be ignored
// without error.
const sampleSessionJSON = `{
	"user_id": "abd99dd7-1ca4-4faa-94e6-669882d60d31",
	"username": "arifinyunianta",
	"email": "arifinyunianta@uzeet360.com",
	"name": "Arifin Yunianta",
	"profile_image": "",
	"phone_number": "",
	"key_data": "ses-data-abd99dd7-1ca4-4faa-94e6-669882d60d31",
	"company": {
		"id": "3c0a1e2e-1111-4a2b-9c3d-4e5f6a7b8c9d",
		"code": "UZT",
		"name": "Uzzeet",
		"apps": [
			{
				"portals": [
					{"id": "portal-1", "permission_type": "role_access", "menus": [], "roles": []}
				]
			}
		]
	}
}`

func TestSessionEnvelope_ParsesCompanyIdAndIgnoresRest(t *testing.T) {
	var session SessionEnvelope
	if err := json.Unmarshal([]byte(sampleSessionJSON), &session); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	want := "3c0a1e2e-1111-4a2b-9c3d-4e5f6a7b8c9d"
	if session.Company.Id != want {
		t.Fatalf("Company.Id = %q, want %q", session.Company.Id, want)
	}
}

func TestSessionEnvelope_EmptyCompanyOnMissingField(t *testing.T) {
	var session SessionEnvelope
	if err := json.Unmarshal([]byte(`{"user_id":"x"}`), &session); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if session.Company.Id != "" {
		t.Fatalf("Company.Id = %q, want empty", session.Company.Id)
	}
}
