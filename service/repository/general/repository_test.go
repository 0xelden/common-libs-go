package general_test

import (
	"reflect"
	"testing"

	"github.com/0xelden/common-libs-go/service/repository/general"
)

type TestDTO struct {
	ID   string
	Name string
}

type testDTOClone struct {
	ID   string
	Name string
}

type testModelEmbedded struct {
	TestDTO
	Extra string
}

type testModelEmbeddedPtr struct {
	*TestDTO
	Extra string
}

func TestRepository_ToDto(t *testing.T) {
	t.Run("same type", func(t *testing.T) {
		repo := &general.Repository[TestDTO, TestDTO]{}
		input := TestDTO{ID: "1", Name: "alpha"}
		got := repo.ToDto(input)
		if got != input {
			t.Fatalf("expected %+v, got %+v", input, got)
		}
	})

	t.Run("embedded dto", func(t *testing.T) {
		repo := &general.Repository[TestDTO, testModelEmbedded]{}
		input := testModelEmbedded{
			TestDTO: TestDTO{ID: "2", Name: "beta"},
			Extra:   "x",
		}
		got := repo.ToDto(input)
		if got != input.TestDTO {
			t.Fatalf("expected %+v, got %+v", input.TestDTO, got)
		}
	})

	t.Run("pointer to dto", func(t *testing.T) {
		repo := &general.Repository[TestDTO, *TestDTO]{}
		input := &TestDTO{ID: "3", Name: "gamma"}
		got := repo.ToDto(input)
		if got != *input {
			t.Fatalf("expected %+v, got %+v", *input, got)
		}
	})

	t.Run("dto pointer type", func(t *testing.T) {
		repo := &general.Repository[*TestDTO, *TestDTO]{}
		input := &TestDTO{ID: "4", Name: "delta"}
		got := repo.ToDto(input)
		if got != input {
			t.Fatalf("expected %+v, got %+v", input, got)
		}
	})

	t.Run("embedded pointer dto", func(t *testing.T) {
		repo := &general.Repository[TestDTO, testModelEmbeddedPtr]{}
		input := testModelEmbeddedPtr{
			TestDTO: &TestDTO{ID: "5", Name: "epsilon"},
			Extra:   "x",
		}
		got := repo.ToDto(input)
		if got != *input.TestDTO {
			t.Fatalf("expected %+v, got %+v", *input.TestDTO, got)
		}
	})

	t.Run("embedded pointer dto nil", func(t *testing.T) {
		repo := &general.Repository[TestDTO, testModelEmbeddedPtr]{}
		input := testModelEmbeddedPtr{TestDTO: nil}
		var zero TestDTO
		got := repo.ToDto(input)
		if got != zero {
			t.Fatalf("expected %+v, got %+v", zero, got)
		}
	})

	t.Run("convertible struct", func(t *testing.T) {
		repo := &general.Repository[TestDTO, testDTOClone]{}
		input := testDTOClone{ID: "6", Name: "zeta"}
		got := repo.ToDto(input)
		if got != TestDTO(input) {
			t.Fatalf("expected %+v, got %+v", TestDTO(input), got)
		}
	})

	t.Run("incompatible type", func(t *testing.T) {
		repo := &general.Repository[TestDTO, int]{}
		input := 7
		var zero TestDTO
		got := repo.ToDto(input)
		if got != zero {
			t.Fatalf("expected %+v, got %+v", zero, got)
		}
	})
}

func TestRepository_ToDtoList(t *testing.T) {
	repo := &general.Repository[TestDTO, testModelEmbedded]{}
	src := []testModelEmbedded{
		{TestDTO: TestDTO{ID: "1", Name: "alpha"}, Extra: "x"},
		{TestDTO: TestDTO{ID: "2", Name: "beta"}, Extra: "y"},
	}
	wantDefault := []TestDTO{src[0].TestDTO, src[1].TestDTO}

	t.Run("fn omitted uses ToDto", func(t *testing.T) {
		got := repo.ToDtoList(src)
		if !reflect.DeepEqual(got, wantDefault) {
			t.Fatalf("expected %+v, got %+v", wantDefault, got)
		}
	})

	t.Run("fn nil uses ToDto", func(t *testing.T) {
		var fn func(testModelEmbedded) TestDTO
		got := repo.ToDtoList(src, fn)
		if !reflect.DeepEqual(got, wantDefault) {
			t.Fatalf("expected %+v, got %+v", wantDefault, got)
		}
	})

	t.Run("fn provided uses fn", func(t *testing.T) {
		got := repo.ToDtoList(src, func(m testModelEmbedded) TestDTO {
			return TestDTO{ID: m.ID + "-custom", Name: m.Name + "-custom"}
		})
		want := []TestDTO{
			{ID: "1-custom", Name: "alpha-custom"},
			{ID: "2-custom", Name: "beta-custom"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})
}
