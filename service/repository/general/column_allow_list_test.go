package general_test

import (
	"context"
	"testing"

	"github.com/0xelden/common-libs-go/api"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/service/repository/general"
)

type stubLister struct{ called bool }

func (s *stubLister) List(context.Context, api.IndexParam) (api.RowIndex[TestDTO], serror.SError) {
	s.called = true
	return api.RowIndex[TestDTO]{}, nil
}

func indexParam(t *testing.T, filter string, sort ...string) api.IndexParam {
	t.Helper()
	param, err := api.NewIndexParamWithContext(context.Background(), &api.IndexParam{Filter: filter, Sort: sort})
	if err != nil {
		t.Fatalf("NewIndexParamWithContext: %v", err)
	}
	return *param
}

func TestRepositoryListColumnAllowList(t *testing.T) {
	lister := &stubLister{}
	repo := general.NewRepositoryBuilder[TestDTO, TestDTO](nil, "t").
		WithOverride(lister).
		WithColumnAllowList([]string{"id", "name"}, []string{"name"}).
		New()

	if _, serr := repo.List(context.Background(), indexParam(t, "result.password_hash~'$2a%'")); serr == nil {
		t.Fatal("filter on hidden column was accepted")
	}
	if _, serr := repo.List(context.Background(), indexParam(t, "", "password_hash:asc")); serr == nil {
		t.Fatal("sort on hidden column was accepted")
	}
	if lister.called {
		t.Fatal("List ran despite a rejected column")
	}
	if _, serr := repo.List(context.Background(), indexParam(t, "result.name~'a%'", "name:desc")); serr != nil {
		t.Fatalf("allowed columns rejected: %v", serr)
	}
	if !lister.called {
		t.Fatal("List did not run for allowed columns")
	}
}

func TestRepositoryListWithoutAllowListIsUnrestricted(t *testing.T) {
	repo := general.NewRepositoryBuilder[TestDTO, TestDTO](nil, "t").WithOverride(&stubLister{}).New()
	if _, serr := repo.List(context.Background(), indexParam(t, "result.password_hash~'x'", "password_hash:asc")); serr != nil {
		t.Fatalf("unrestricted repository rejected a column: %v", serr)
	}
}
