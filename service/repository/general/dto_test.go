package general

import (
	"context"
	"testing"

	"github.com/0xelden/common-libs-go/helper"
)

type uuidIdDto struct {
	Id   string `db:"id"`
	Name string `db:"name"`
}

type bigintIdDto struct {
	Id   int64  `db:"id"`
	Name string `db:"name"`
}

type bigintPtrIdDto struct {
	Id   *int64 `db:"id"`
	Name string `db:"name"`
}

type noIdDto struct {
	Name string `db:"name"`
}

func TestPrepStructInsertId(t *testing.T) {
	ctx := context.Background()
	existing := "6a01dbe8-59e3-4f1b-9e16-03c4c9977ff3"
	seven := int64(7)

	t.Run("uuid id is generated when empty", func(t *testing.T) {
		maps, serr := NewDTO[uuidIdDto]("t", nil).PrepStructInsert(ctx, &uuidIdDto{})
		if serr != nil {
			t.Fatal(serr)
		}
		if !helper.IsValidUUID(helper.ToString(maps["id"])) {
			t.Fatalf("expected generated uuid, got %#v", maps["id"])
		}
	})

	t.Run("uuid id is kept when valid", func(t *testing.T) {
		maps, _ := NewDTO[uuidIdDto]("t", nil).PrepStructInsert(ctx, &uuidIdDto{Id: existing})
		if maps["id"] != existing {
			t.Fatalf("expected %s, got %#v", existing, maps["id"])
		}
	})

	t.Run("missing id field still gets uuid", func(t *testing.T) {
		maps, _ := NewDTO[noIdDto]("t", nil).PrepStructInsert(ctx, &noIdDto{})
		if !helper.IsValidUUID(helper.ToString(maps["id"])) {
			t.Fatalf("expected generated uuid, got %#v", maps["id"])
		}
	})

	t.Run("numeric zero id is omitted", func(t *testing.T) {
		maps, _ := NewDTO[bigintIdDto]("t", nil).PrepStructInsert(ctx, &bigintIdDto{})
		if _, ok := maps["id"]; ok {
			t.Fatalf("expected id to be omitted, got %#v", maps["id"])
		}
	})

	t.Run("numeric nil pointer id is omitted", func(t *testing.T) {
		maps, _ := NewDTO[bigintPtrIdDto]("t", nil).PrepStructInsert(ctx, &bigintPtrIdDto{})
		if _, ok := maps["id"]; ok {
			t.Fatalf("expected id to be omitted, got %#v", maps["id"])
		}
	})

	t.Run("numeric id is kept when set", func(t *testing.T) {
		maps, _ := NewDTO[bigintIdDto]("t", nil).PrepStructInsert(ctx, &bigintIdDto{Id: 7})
		if maps["id"] != int64(7) {
			t.Fatalf("expected 7, got %#v", maps["id"])
		}
		maps, _ = NewDTO[bigintPtrIdDto]("t", nil).PrepStructInsert(ctx, &bigintPtrIdDto{Id: &seven})
		if maps["id"] != &seven {
			t.Fatalf("expected pointer to 7, got %#v", maps["id"])
		}
	})
}
