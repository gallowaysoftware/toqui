package handlers

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type PersonaHandler struct {
	registry *persona.Registry
	queries  *dbgen.Queries
}

func NewPersonaHandler(registry *persona.Registry, pool *pgxpool.Pool) *PersonaHandler {
	return &PersonaHandler{
		registry: registry,
		queries:  dbgen.New(pool),
	}
}

func (h *PersonaHandler) ListPersonas(ctx context.Context, req *connect.Request[toquiv1.ListPersonasRequest]) (*connect.Response[toquiv1.ListPersonasResponse], error) {
	all := h.registry.ListAll()

	var personas []*toquiv1.Persona
	for _, p := range all {
		proto := personaToProto(p)

		// Apply filters
		if req.Msg.Type != toquiv1.PersonaType_PERSONA_TYPE_UNSPECIFIED && proto.Type != req.Msg.Type {
			continue
		}
		if req.Msg.RegionCode != "" && proto.RegionCode != req.Msg.RegionCode {
			continue
		}

		personas = append(personas, proto)
	}

	return connect.NewResponse(&toquiv1.ListPersonasResponse{Personas: personas}), nil
}

func (h *PersonaHandler) GetPersona(ctx context.Context, req *connect.Request[toquiv1.GetPersonaRequest]) (*connect.Response[toquiv1.GetPersonaResponse], error) {
	p, err := h.registry.Get(req.Msg.PersonaId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&toquiv1.GetPersonaResponse{Persona: personaToProto(p)}), nil
}

func (h *PersonaHandler) SetDefaultPersona(ctx context.Context, req *connect.Request[toquiv1.SetDefaultPersonaRequest]) (*connect.Response[toquiv1.SetDefaultPersonaResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	p, err := h.registry.Get(req.Msg.PersonaId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	_, err = h.queries.SetUserDefaultPersona(ctx, dbgen.SetUserDefaultPersonaParams{
		ID:               userID,
		DefaultPersonaID: pgtype.Text{String: req.Msg.PersonaId, Valid: true},
	})
	if err != nil {
		return nil, internalError(ctx, "persona operation", err)
	}

	return connect.NewResponse(&toquiv1.SetDefaultPersonaResponse{Persona: personaToProto(p)}), nil
}

func (h *PersonaHandler) ResolvePersona(ctx context.Context, req *connect.Request[toquiv1.ResolvePersonaRequest]) (*connect.Response[toquiv1.ResolvePersonaResponse], error) {
	// Determine region code from the trip's destination_country. The region
	// code (ISO 3166-1 alpha-2: "JP", "FR", "US", …) is what
	// persona.Registry.Resolve uses to pick a location-specific expert
	// (e.g. Japan + food → Akari, the Tokyo izakaya guide).
	//
	// Pre-fix the handler hard-coded `regionCode = ""` which meant every
	// resolution fell through to a theme-only match, ignoring location
	// entirely. So a user planning a trip to Greece with a "food" theme
	// would get whichever generic food expert ranked first, not the
	// Greek-cuisine specialist. Closes the TODO that was here.
	//
	// Best-effort lookup: a missing trip / non-owner / DB error all
	// degrade gracefully to the empty regionCode + theme-only resolution
	// (the prior behaviour). We deliberately do NOT return an error
	// here — chat-flow callers can be in a partial-state during trip
	// switching, and a hard error would disrupt the conversation.
	regionCode := ""
	if userID, ok := auth.UserIDFromContext(ctx); ok && req.Msg.TripId != "" {
		if tripID, err := uuid.Parse(req.Msg.TripId); err == nil {
			t, err := h.queries.GetTripByIDOrCollaborator(ctx, dbgen.GetTripByIDOrCollaboratorParams{
				ID:     tripID,
				UserID: userID,
			})
			if err == nil && t.DestinationCountry.Valid {
				regionCode = t.DestinationCountry.String
			} else if err != nil {
				slog.Debug("ResolvePersona trip lookup failed, falling back to theme-only resolution",
					"trip_id", tripID, "error", err)
			}
		}
	}

	resolved, err := h.registry.Resolve(ctx, regionCode, req.Msg.Themes)
	if err != nil {
		resolved = h.registry.Default()
	}

	resp := &toquiv1.ResolvePersonaResponse{
		Persona: personaToProto(resolved),
	}

	// Include handoff message if resolved to an expert (not the orchestrator)
	if resolved.ID != "toqui" {
		resp.HandoffMessage = h.registry.HandoffMessage(resolved)
		resp.HandoffFrom = personaToProto(h.registry.Default())
	}

	return connect.NewResponse(resp), nil
}

func personaToProto(p *persona.Persona) *toquiv1.Persona {
	pType := toquiv1.PersonaType_PERSONA_TYPE_GLOBAL
	if p.ID != "toqui" {
		if len(p.Specialties) > 0 {
			pType = toquiv1.PersonaType_PERSONA_TYPE_LOCAL_SPECIALIST
		} else {
			pType = toquiv1.PersonaType_PERSONA_TYPE_LOCAL_GUIDE
		}
	}

	return &toquiv1.Persona{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		AvatarUrl:   p.AvatarURL,
		Greeting:    p.Greeting,
		Type:        pType,
		Specialties: p.Specialties,
		AccentColor: p.AccentColor,
		IsDefault:   p.ID == "toqui",
	}
}
