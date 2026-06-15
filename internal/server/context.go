package server

import "context"

// requestIDKey is an unexported type so no other package can
// collide with the context key. The string form is only for log
// readability; access goes through the helpers below.
type requestIDKey struct{}

func contextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func requestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}
