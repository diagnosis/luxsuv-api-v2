package secure

import (
	"context"
	"errors"
)

type contextKey string

const claimsKey contextKey = "claims"

func ClaimsFromContext(ctx context.Context) (*AccessClaims, error) {
	val := ctx.Value(claimsKey)
	if val == nil {
		return nil, errors.New("no claims in context")
	}
	claims, ok := val.(*AccessClaims)
	if !ok {
		return nil, errors.New("invalid claims type in context")
	}
	return claims, nil
}

func WithClaims(ctx context.Context, c *AccessClaims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}
