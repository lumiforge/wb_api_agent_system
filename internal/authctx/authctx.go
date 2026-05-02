package authctx

import (
	"context"
	"fmt"
	"strings"
)

type userJWTContextKey struct{}

func WithUserJWT(ctx context.Context, userJWT string) context.Context {
	return context.WithValue(ctx, userJWTContextKey{}, strings.TrimSpace(userJWT))
}

func UserJWT(ctx context.Context) (string, error) {
	value, ok := ctx.Value(userJWTContextKey{}).(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("user jwt is required")
	}

	return strings.TrimSpace(value), nil
}

func BearerTokenFromAuthorizationHeader(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", fmt.Errorf("Authorization header is required")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("Authorization header format must be Bearer {token}")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", fmt.Errorf("bearer token is empty")
	}

	return token, nil
}