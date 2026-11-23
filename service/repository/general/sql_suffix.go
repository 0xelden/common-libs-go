package general

import "context"

type sqlSuffixKey struct{}

// WithSqlSuffix appends suffix (e.g. an ON CONFLICT clause) to the statements
// built by InsertRows, PatchRows and PatchRow. The suffix is raw SQL, so pass
// only server-side constants, never request input.
func WithSqlSuffix(ctx context.Context, suffix string) context.Context {
	return context.WithValue(ctx, sqlSuffixKey{}, suffix)
}

func sqlSuffix(ctx context.Context) string {
	suffix, _ := ctx.Value(sqlSuffixKey{}).(string)
	return suffix
}
