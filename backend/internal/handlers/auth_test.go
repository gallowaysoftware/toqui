package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

func TestUserToProto_DefaultsAndOptionals(t *testing.T) {
	uid := uuid.New()
	u := &dbgen.User{
		ID:    uid,
		Email: "u@example.com",
		Name:  pgtype.Text{Valid: false},
	}

	proto := userToProto(u)
	if proto.Name != "" {
		t.Errorf("Name = %q, want empty when invalid", proto.Name)
	}
	if proto.AvatarUrl != "" {
		t.Errorf("AvatarUrl = %q, want empty when invalid", proto.AvatarUrl)
	}

	// Populated optional fields.
	u.Name = pgtype.Text{String: "Alice", Valid: true}
	u.AvatarUrl = pgtype.Text{String: "https://example.com/a.png", Valid: true}
	proto2 := userToProto(u)
	if proto2.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", proto2.Name)
	}
	if proto2.AvatarUrl != "https://example.com/a.png" {
		t.Errorf("AvatarUrl = %q, want url", proto2.AvatarUrl)
	}
}
