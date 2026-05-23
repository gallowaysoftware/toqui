package auth

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

var publicMethods = map[string]bool{
	"/toqui.v1.AuthService/GoogleLogin":      true,
	"/toqui.v1.AuthService/EmailRegister":    true,
	"/toqui.v1.AuthService/EmailLogin":       true,
	"/toqui.v1.AuthService/GetAuthProviders": true,
	"/toqui.v1.AuthService/RefreshToken":     true,
}

// authInterceptor implements connect.Interceptor for both unary and streaming RPCs.
type authInterceptor struct {
	authSvc *Service
}

func NewAuthInterceptor(authSvc *Service) *authInterceptor {
	return &authInterceptor{authSvc: authSvc}
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if publicMethods[req.Spec().Procedure] {
			return next(ctx, req)
		}

		userID, err := extractUserID(req.Header(), i.authSvc)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}

		ctx = ContextWithUserID(ctx, userID)
		return next(ctx, req)
	}
}

func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // client-side: no-op on the server
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if publicMethods[conn.Spec().Procedure] {
			return next(ctx, conn)
		}

		userID, err := extractUserID(conn.RequestHeader(), i.authSvc)
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}

		ctx = ContextWithUserID(ctx, userID)
		return next(ctx, conn)
	}
}

func extractUserID(headers interface{ Get(string) string }, authSvc *Service) (uuid.UUID, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return uuid.Nil, fmt.Errorf("missing authorization header")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	return authSvc.ValidateToken(token)
}
