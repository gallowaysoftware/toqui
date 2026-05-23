# Persona: Structural — Collaboration Lifecycle

## Background

This is a structural test that exercises the trip collaboration system. It tests the full invite → accept → collaborate → remove lifecycle. This test requires TWO test users — create both via testctl before starting.

## Your Trip

A minimal trip used for collaboration testing. Title: "Collab Test — Italy Group." Destination: Italy.

## What to Test

**Setup**: You need two test users. The orchestrator should create both and provide both tokens. User A is the trip owner, User B is the collaborator.

Execute as User A (owner):
1. **CreateTrip**: Create "Collab Test — Italy Group" with destination IT.
2. **Add itinerary items**: Send a chat message to add 3 items to the itinerary. Verify via GetItinerary.
3. **Invite User B**: Call `POST /api/trips/{id}/invite` with User B's email and role "editor".
4. **List collaborators**: Call `GET /api/trips/{id}/collaborators`. Verify User B appears as invited (not yet accepted).

Execute as User B (collaborator):
5. **Accept invite**: Call `POST /api/trips/accept-invite` with the invite token.
6. **GetTrip**: Verify User B can access the trip via `GetTrip` or `GetByIDOrCollaborator`.
7. **GetItinerary**: Verify User B can see the 3 itinerary items created by User A.
8. **Chat as collaborator**: Send a chat message like "Add a visit to the Colosseum on day 2." Verify the AI creates itinerary items (editor role should allow this).
9. **Verify itinerary updated**: Call `GetItinerary` and verify the new items from User B's chat appear alongside User A's items.

Execute as User A (owner):
10. **Verify User B's additions**: GetItinerary should show both User A's and User B's items.
11. **Remove collaborator**: Call `DELETE /api/trips/{id}/collaborators/{userB_id}`.
12. **Verify removal**: `GET /api/trips/{id}/collaborators` should show empty list.

Execute as User B (removed):
13. **Verify access revoked**: `GetTrip` for the trip should fail with not found or permission denied.

## Special Attention

- **Editor can modify itinerary**: This is the core test for #263. If User B cannot add itinerary items via chat, that's a P1 regression.
- **Viewer cannot modify**: If time permits, test inviting a second user as "viewer" and verify they can read but not modify.
- **Owner retains control**: User A should always be able to modify regardless of collaborator state.
- **Removal is immediate**: After removal, User B should have zero access.
