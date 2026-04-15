package analytics

// Funnel event names for tracking user progression through the product.
// These fire once per entity (user/trip) to measure conversion rates.
const (
	EventUserActivated        = "user_activated"        // First trip created
	EventItineraryCompleted   = "itinerary_completed"   // Trip has 5+ itinerary items
	EventBookingAdded         = "booking_added"         // First booking ingested for a trip
	EventTripCompleted        = "trip_completed"        // Trip status → completed
	EventExportUsed           = "export_used"           // iCal or PDF exported
	EventCollaborationStarted = "collaboration_started" // First collaborator invited
)

// TrackUserActivated fires when a user creates their first trip.
// Should be called only once per user (check trip count before calling).
func (c *Client) TrackUserActivated(userID string) {
	c.Track(userID, EventUserActivated, nil)
}

// TrackItineraryCompleted fires when a trip reaches 5+ itinerary items.
// Indicates the user is getting real value from the planning experience.
func (c *Client) TrackItineraryCompleted(userID, tripID string, itemCount int) {
	c.Track(userID, EventItineraryCompleted, map[string]any{
		"trip_id":    tripID,
		"item_count": itemCount,
	})
}

// TrackBookingAdded fires when a booking is ingested for a trip.
func (c *Client) TrackBookingAdded(userID, tripID, bookingType string) {
	c.Track(userID, EventBookingAdded, map[string]any{
		"trip_id":      tripID,
		"booking_type": bookingType,
	})
}

// TrackTripCompleted fires when a trip transitions to completed status.
func (c *Client) TrackTripCompleted(userID, tripID string, dayCount int) {
	c.Track(userID, EventTripCompleted, map[string]any{
		"trip_id":   tripID,
		"day_count": dayCount,
	})
}

// TrackExportUsed fires when a user exports their itinerary.
func (c *Client) TrackExportUsed(userID, tripID, format string) {
	c.Track(userID, EventExportUsed, map[string]any{
		"trip_id": tripID,
		"format":  format, // "ical" or "pdf"
	})
}

// TrackCollaborationStarted fires when the first collaborator is invited.
func (c *Client) TrackCollaborationStarted(userID, tripID string) {
	c.Track(userID, EventCollaborationStarted, map[string]any{
		"trip_id": tripID,
	})
}
