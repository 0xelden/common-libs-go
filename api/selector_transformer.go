package api

import (
	"context"
)

type Selector interface {
	IndexParam | ViewParam
}

type Transformer[T Selector] interface {
	Transform(ctx context.Context, param T) (res any, err error)
}
