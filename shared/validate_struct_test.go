package shared

import (
	"testing"

	"github.com/gookit/validate/v2"
)

// sceneDto/sceneModel mirror the generated module shape: a Dto embedded in the
// request model, with per-scene required fields declared by ConfigValidation.
type sceneDto struct {
	Name  string `form:"name" json:"name"`
	Email string `form:"email" json:"email"`
}

type sceneAddress struct {
	City string `json:"city" validate:"required"`
}

type sceneModel struct {
	sceneDto
	Address sceneAddress `json:"address"`
}

func (sceneModel) ConfigValidation(v *validate.Validation) {
	v.WithScenes(validate.SValues{
		SceneCreate: []string{"Name", "Email", "Address.City"},
		SceneEdit:   []string{"Email"},
	})
	v.AddRule("Name", "required")
}

func TestValidateStructAppliesSceneRules(t *testing.T) {
	ok := sceneModel{sceneDto: sceneDto{Name: "a", Email: "a@b.co"}, Address: sceneAddress{City: "x"}}
	if err := ValidateStruct(SceneCreate, &ok); err != nil {
		t.Fatalf("valid model rejected: %v", err)
	}

	// ConfigValidation must be picked up: without it Name has no rule at all.
	missingName := sceneModel{sceneDto: sceneDto{Email: "a@b.co"}, Address: sceneAddress{City: "x"}}
	if err := ValidateStruct(SceneCreate, &missingName); err == nil {
		t.Fatal("missing Name passed: ConfigValidation was not applied")
	}

	// v1 cascaded into untagged named sub-structs; v2 only does so with
	// CheckSubOnParentMarked disabled (see init).
	noCity := sceneModel{sceneDto: sceneDto{Name: "a", Email: "a@b.co"}}
	if err := ValidateStruct(SceneCreate, &noCity); err == nil {
		t.Fatal("missing Address.City passed: sub-struct rules were not applied")
	}

	// The Edit scene only checks Email, so a missing Name is fine there.
	if err := ValidateStruct(SceneEdit, &missingName); err != nil {
		t.Fatalf("edit scene rejected a model it does not cover: %v", err)
	}
}

type AuditFields struct {
	CreatedBy *string `json:"created_by" validate:"required"`
}

// EmbedDto carries its rules as tags, exactly like a generated <Entity>Dto.
type EmbedDto struct {
	AuditFields
	Id    string  `json:"id" validate:"required|uuid"`
	Title *string `json:"title" validate:"required"`
	Email *string `json:"email" validate:"email"`
}

type embedModel struct {
	EmbedDto
	Note string `json:"note"`
}

func (embedModel) ConfigValidation(v *validate.Validation) {
	v.WithScenes(validate.SValues{
		SceneCreate: []string{"Title", "Email", "CreatedBy"},
		SceneEdit:   []string{"Id"},
	})
}

// Rules on fields promoted from an embedded Dto are registered under
// "EmbedDto.Title", not "Title"; the scene must still select them.
func TestValidateStructEnforcesEmbeddedSceneFields(t *testing.T) {
	title, email, bad, by := "t", "a@b.co", "nope", "u1"
	id := "6f1c2a8e-3b7d-4c1e-9a2f-0d5e8b7c6a41"

	cases := []struct {
		name    string
		scene   string
		model   embedModel
		wantErr bool
	}{
		{"create valid", SceneCreate, embedModel{EmbedDto: EmbedDto{Title: &title, Email: &email, AuditFields: AuditFields{CreatedBy: &by}}}, false},
		{"create missing title", SceneCreate, embedModel{EmbedDto: EmbedDto{Email: &email, AuditFields: AuditFields{CreatedBy: &by}}}, true},
		{"create bad email", SceneCreate, embedModel{EmbedDto: EmbedDto{Title: &title, Email: &bad, AuditFields: AuditFields{CreatedBy: &by}}}, true},
		{"create missing nested embedded field", SceneCreate, embedModel{EmbedDto: EmbedDto{Title: &title, Email: &email}}, true},
		// Id is not in the Create scene, so it must not be required there.
		{"create without id", SceneCreate, embedModel{EmbedDto: EmbedDto{Title: &title, AuditFields: AuditFields{CreatedBy: &by}}}, false},
		{"edit valid", SceneEdit, embedModel{EmbedDto: EmbedDto{Id: id}}, false},
		{"edit missing id", SceneEdit, embedModel{}, true},
		{"edit bad id", SceneEdit, embedModel{EmbedDto: EmbedDto{Id: "x"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(tc.scene, &tc.model)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
