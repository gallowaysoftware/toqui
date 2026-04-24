package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

func TestUserToProto_AgeVerifiedAt(t *testing.T) {
	uid := uuid.New()
	verifiedAt := time.Date(2026, 4, 20, 12, 30, 0, 0, time.UTC)

	cases := []struct {
		name          string
		ageVerifiedAt pgtype.Timestamptz
		wantUnset     bool
		wantTime      time.Time
	}{
		{
			name:          "unverified user - invalid zero value",
			ageVerifiedAt: pgtype.Timestamptz{},
			wantUnset:     true,
		},
		{
			name:          "unverified user - valid=false with non-zero time",
			ageVerifiedAt: pgtype.Timestamptz{Time: verifiedAt, Valid: false},
			wantUnset:     true,
		},
		{
			name:          "verified user - populated timestamp",
			ageVerifiedAt: pgtype.Timestamptz{Time: verifiedAt, Valid: true},
			wantUnset:     false,
			wantTime:      verifiedAt,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &dbgen.User{
				ID:            uid,
				Email:         "test@example.com",
				Name:          pgtype.Text{String: "Test User", Valid: true},
				AgeVerifiedAt: tc.ageVerifiedAt,
			}
			proto := userToProto(u, "free")

			if proto == nil {
				t.Fatal("userToProto returned nil")
			}
			if proto.Id != uid.String() {
				t.Errorf("Id = %q, want %q", proto.Id, uid.String())
			}
			if proto.Email != "test@example.com" {
				t.Errorf("Email = %q, want test@example.com", proto.Email)
			}

			if tc.wantUnset {
				if proto.AgeVerifiedAt != nil {
					t.Errorf("AgeVerifiedAt = %v, want nil for unverified user", proto.AgeVerifiedAt)
				}
				return
			}

			if proto.AgeVerifiedAt == nil {
				t.Fatalf("AgeVerifiedAt = nil, want populated timestamp")
			}
			got := proto.AgeVerifiedAt.AsTime()
			if !got.Equal(tc.wantTime) {
				t.Errorf("AgeVerifiedAt = %v, want %v", got, tc.wantTime)
			}
		})
	}
}

func TestUserToProto_DefaultsAndOptionals(t *testing.T) {
	uid := uuid.New()
	u := &dbgen.User{
		ID:    uid,
		Email: "u@example.com",
		Name:  pgtype.Text{Valid: false},
	}

	// Empty subscription tier should default to "free".
	proto := userToProto(u, "")
	if proto.SubscriptionTier != "free" {
		t.Errorf("SubscriptionTier = %q, want free", proto.SubscriptionTier)
	}
	if proto.Name != "" {
		t.Errorf("Name = %q, want empty when invalid", proto.Name)
	}
	if proto.AvatarUrl != "" {
		t.Errorf("AvatarUrl = %q, want empty when invalid", proto.AvatarUrl)
	}
	if proto.AgeVerifiedAt != nil {
		t.Errorf("AgeVerifiedAt = %v, want nil when invalid", proto.AgeVerifiedAt)
	}

	// Populated optional fields.
	u.Name = pgtype.Text{String: "Alice", Valid: true}
	u.AvatarUrl = pgtype.Text{String: "https://example.com/a.png", Valid: true}
	proto2 := userToProto(u, "pro")
	if proto2.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", proto2.Name)
	}
	if proto2.AvatarUrl != "https://example.com/a.png" {
		t.Errorf("AvatarUrl = %q, want url", proto2.AvatarUrl)
	}
	if proto2.SubscriptionTier != "pro" {
		t.Errorf("SubscriptionTier = %q, want pro", proto2.SubscriptionTier)
	}
}
