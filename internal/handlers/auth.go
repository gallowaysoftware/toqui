package handlers

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type AuthHandler struct {
	authSvc      *auth.Service
	queries      *dbgen.Queries
	lifecycleSvc *lifecycle.Service
}

func NewAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, lifecycleSvc *lifecycle.Service) *AuthHandler {
	return &AuthHandler{
		authSvc:      authSvc,
		queries:      dbgen.New(pool),
		lifecycleSvc: lifecycleSvc,
	}
}

func (h *AuthHandler) GoogleLogin(ctx context.Context, req *connect.Request[toquiv1.GoogleLoginRequest]) (*connect.Response[toquiv1.GoogleLoginResponse], error) {
	info, err := h.authSvc.ExchangeCode(ctx, req.Msg.Code)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	user, err := h.queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID:  info.ID,
		Email:     info.Email,
		Name:      pgtype.Text{String: info.Name, Valid: info.Name != ""},
		AvatarUrl: pgtype.Text{String: info.AvatarURL, Valid: info.AvatarURL != ""},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	refreshToken, err := h.authSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.GoogleLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         userToProto(&user),
	}), nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *connect.Request[toquiv1.RefreshTokenRequest]) (*connect.Response[toquiv1.RefreshTokenResponse], error) {
	userID, err := h.authSvc.ValidateRefreshToken(req.Msg.RefreshToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	refreshToken, err := h.authSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         userToProto(&user),
	}), nil
}

func (h *AuthHandler) GetCurrentUser(ctx context.Context, _ *connect.Request[toquiv1.GetCurrentUserRequest]) (*connect.Response[toquiv1.GetCurrentUserResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.GetCurrentUserResponse{
		User: userToProto(&user),
	}), nil
}

func (h *AuthHandler) DeleteAccount(ctx context.Context, req *connect.Request[toquiv1.DeleteAccountRequest]) (*connect.Response[toquiv1.DeleteAccountResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	if !req.Msg.Confirm {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("must set confirm=true to delete account"))
	}

	requestID, err := h.lifecycleSvc.RequestDeletion(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.DeleteAccountResponse{
		RequestId: requestID.String(),
		Message:   "Your account and all associated data have been permanently deleted.",
	}), nil
}

func (h *AuthHandler) ExportData(ctx context.Context, _ *connect.Request[toquiv1.ExportDataRequest]) (*connect.Response[toquiv1.ExportDataResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	requestID, err := h.lifecycleSvc.RequestExport(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.ExportDataResponse{
		RequestId: requestID.String(),
		Message:   "Your data export is being prepared. You'll be notified when it's ready to download.",
	}), nil
}

func userToProto(u *dbgen.User) *toquiv1.User {
	user := &toquiv1.User{
		Id:    u.ID.String(),
		Email: u.Email,
	}
	if u.Name.Valid {
		user.Name = u.Name.String
	}
	if u.AvatarUrl.Valid {
		user.AvatarUrl = u.AvatarUrl.String
	}
	return user
}
