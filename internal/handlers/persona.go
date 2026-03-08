package handlers

import (
	"context"

	"connectrpc.com/connect"
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
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.SetDefaultPersonaResponse{Persona: personaToProto(p)}), nil
}

func (h *PersonaHandler) ResolvePersona(ctx context.Context, req *connect.Request[toquiv1.ResolvePersonaRequest]) (*connect.Response[toquiv1.ResolvePersonaResponse], error) {
	// Determine region code from trip context or coordinates
	regionCode := ""
	// TODO: The caller should pass region code via the trip's destination_country.
	// For now, we rely on the themes list being sufficient for resolution.
	// Location-based resolution (lat/lng → region code) is a future enhancement.

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
