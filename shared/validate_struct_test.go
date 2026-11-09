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
