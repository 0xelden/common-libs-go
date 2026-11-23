package auth

import (
	"encoding/json"
	"testing"
)

const sampleSessionJSON = `{
	"user_id": "abd99dd7-1ca4-4faa-94e6-669882d60d31",
	"username": "johndoe",
	"email": "johndoe@example.com",
	"name": "John Doe",
	"profile_image": "",
	"phone_number": "",
	"key_data": "ses-data-abd99dd7-1ca4-4faa-94e6-669882d60d31",
	"company": {
		"id": "3c0a1e2e-1111-4a2b-9c3d-4e5f6a7b8c9d",
		"code": "ACME",
		"name": "Acme",
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
