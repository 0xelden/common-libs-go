package db

import (
	"context"

	"github.com/0xelden/common-libs-go/shared"
)

func ExecuteIn(ctx context.Context, connection string) context.Context {
	return context.WithValue(ctx, shared.ExecuteIn, connection)
}
