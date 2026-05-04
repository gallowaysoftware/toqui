package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

// AppleLogin handles the Apple Sign-In flow.
//
// Flow:
//  1. Exchange the authorization_code with Apple for tokens.
//  2. Verify the ID token returned by the exchange (we trust the exchange's
//     id_token over the one supplied by the client to defeat replay).
//  3. Resolve the user:
//     - Look up by Apple `sub` → existing user.
//     - Else look up by email → link Apple to existing account.
//     - Else create a new user (subject to capacity cap).
//  4. Issue Toqui access + refresh tokens.
//
// Returns Unimplemented when Apple is not configured (no team ID / services
// ID / key ID / private key). This lets us ship the scaffold before Apple
// Developer enrollment is complete.
func (h *AuthHandler) AppleLogin(ctx context.Context, req *connect.Request[toquiv1.AppleLoginRequest]) (*connect.Response[toquiv1.AppleLoginResponse], error) {
	appleClient := h.authSvc.AppleClient()
	if appleClient == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("apple sign-in not configured"))
	}

	// 1. Exchange the code with Apple.
	tokenResp, err := appleClient.ExchangeCode(ctx, req.Msg.AuthorizationCode, req.Msg.RedirectUri)
	if err != nil {
		slog.Warn("apple code exchange failed", "error", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid apple authorization code"))
	}

	// 2. Verify the ID token returned by Apple. We deliberately ignore the
	// id_token the client provided in the request — the exchange's id_token
	// is bound to the just-redeemed code and can't be replayed.
	claims, err := appleClient.VerifyIDToken(ctx, tokenResp.IDToken)
	if err != nil {
		slog.Warn("apple id token verify failed", "error", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid apple id token"))
	}

	// Apple's `sub` is mandatory and stable per (team, user).
	if claims.Subject == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("apple id token missing subject"))
	}

	// Apple omits `email` on subsequent logins — only the first sign-in
	// includes it. Use whatever Apple gave us, but don't fail when blank.
	email := claims.Email
	if email != "" {
		// Domain allowlist applies on first sign-in only (when we have an
		// email). Returning users (no email) skip this — by then they're
		// already in our system from a prior accepted sign-in.
		if !isEmailDomainAllowed(email, h.allowedDomains) {
			audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(email))
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("email domain not allowed"))
		}

		// Under-age refusal cache (first-sign-in only — the helper
		// short-circuits on empty email anyway, but the gate is most
		// effective on the first time Apple ships us the email).
		if err := checkUnderAgeBlock(ctx, h.queries, email, "apple"); err != nil {
			return nil, err
		}
	}

	user, isNew, err := h.findOrCreateAppleUser(ctx, claims.Subject, email)
	if err != nil {
		if errors.Is(err, errAtCapacity) {
			audit.Log(audit.EventLoginDeniedCapacity, "email", maskEmail(email))
			return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("service at capacity"))
		}
		return nil, internalError(ctx, "apple user upsert", err)
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, internalError(ctx, "generate access token", err)
	}

	refreshResult, err := h.authSvc.GenerateRefreshToken(user.ID, uuid.Nil)
	if err != nil {
		return nil, internalError(ctx, "generate refresh token", err)
	}

	if _, err := h.queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       refreshResult.JTI,
		Family:    refreshResult.Family,
		ExpiresAt: refreshResult.ExpiresAt,
	}); err != nil {
		return nil, internalError(ctx, "store refresh token", err)
	}

	if isNew {
		audit.Log(audit.EventAppleLoginNew, "user_id", user.ID.String(), "email", maskEmail(user.Email))
	} else {
		audit.Log(audit.EventAppleLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email))
	}

	tier := h.lookupTier(ctx, user.ID)

	consentPending := true
	if hasRequired, cErr := h.queries.HasRequiredConsents(ctx, user.ID); cErr != nil {
		slog.Warn("failed to check required consents, assuming pending", "user_id", user.ID, "error", cErr)
	} else {
		consentPending = !hasRequired
	}

	return connect.NewResponse(&toquiv1.AppleLoginResponse{
		User:                    userToProto(user, tier),
		AccessToken:             accessToken,
		RefreshToken:            refreshResult.Token,
		ExpiresAt:               timestamppb.New(refreshResult.ExpiresAt),
		ConsentPending:          consentPending,
		AgeVerificationRequired: !user.AgeVerifiedAt.Valid,
	}), nil
}

// errAtCapacity is the sentinel returned by findOrCreateAppleUser when the
// free-user cap is hit and the email isn't on the allowlist or pre-invited.
var errAtCapacity = errors.New("at capacity")

// findOrCreateAppleUser resolves an Apple sign-in to a Toqui user.
//
// Resolution order:
//  1. Lookup by apple_sub → existing user.
//  2. Lookup by email (when Apple included one) → link apple_sub.
//  3. Create new user, subject to the capacity cap and waitlist invite logic.
//
// The bool return indicates whether a new user was created (true) so the
// caller can route audit logs to the *_new event.
func (h *AuthHandler) findOrCreateAppleUser(ctx context.Context, appleSub, email string) (*dbgen.User, bool, error) {
	subParam := pgtype.Text{String: appleSub, Valid: true}

	// 1. Lookup by apple_sub.
	user, err := h.queries.GetUserByAppleSub(ctx, subParam)
	if err == nil {
		return &user, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("get user by apple_sub: %w", err)
	}

	// 2. If Apple gave us an email, try to link to an existing account.
	if email != "" {
		existing, lookupErr := h.queries.GetUserByEmail(ctx, email)
		if lookupErr == nil {
			if linkErr := h.queries.UpdateUserAppleSub(ctx, dbgen.UpdateUserAppleSubParams{
				ID:       existing.ID,
				AppleSub: subParam,
			}); linkErr != nil {
				return nil, false, fmt.Errorf("link apple_sub to user: %w", linkErr)
			}
			existing.AppleSub = subParam
			audit.Log(audit.EventAppleLink, "user_id", existing.ID.String(), "email", maskEmail(existing.Email))
			return &existing, false, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return nil, false, fmt.Errorf("get user by email: %w", lookupErr)
		}
	}

	// 3. Brand-new user. Apple may not have given us an email on sign-in
	// after the very first one — but we always have an email on first sign-in
	// per Apple's spec, and this branch is only reached on the first sign-in
	// (subsequent sign-ins resolve via apple_sub above). If email is somehow
	// blank here, refuse rather than create a placeholder account.
	if email == "" {
		return nil, false, fmt.Errorf("apple did not return email on first sign-in")
	}

	// Capacity cap (matches Facebook flow exactly).
	if !isEmailAllowListed(email, h.allowedEmails) && h.maxFreeUsers > 0 {
		userCount, countErr := h.queries.CountUsers(ctx)
		if countErr != nil {
			return nil, false, fmt.Errorf("count users: %w", countErr)
		}
		if int(userCount) >= h.maxFreeUsers {
			waitlistEntry, wlErr := h.queries.GetWaitlistByEmail(ctx, email)
			if wlErr != nil || !waitlistEntry.InviteCode.Valid {
				return nil, false, errAtCapacity
			}
			audit.Log(audit.EventLoginAdmittedInvite,
				"email", maskEmail(email),
				"invite_code", waitlistEntry.InviteCode.String,
			)
		}
	}

	// Mark waitlist as accepted (idempotent, no-op for non-listed emails).
	if markErr := h.queries.MarkWaitlistAccepted(ctx, email); markErr != nil && !errors.Is(markErr, pgx.ErrNoRows) {
		slog.Error("mark waitlist accepted on apple signup failed", "email", maskEmail(email), "error", markErr)
	}

	created, err := h.queries.CreateUserWithApple(ctx, dbgen.CreateUserWithAppleParams{
		Email:     email,
		Name:      pgtype.Text{}, // Apple does not return name in the ID token
		AppleSub:  subParam,
		AvatarUrl: pgtype.Text{}, // Apple does not return avatar
	})
	if err != nil {
		return nil, false, fmt.Errorf("create user with apple: %w", err)
	}
	return &created, true, nil
}
