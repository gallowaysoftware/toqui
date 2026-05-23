package ratelimit

import (
	"testing"
)

func TestIsAIProcedure(t *testing.T) {
	if !isAIProcedure("/toqui.v1.ChatService/SendMessage") {
		t.Error("expected SendMessage to be classified as AI procedure")
	}
	if isAIProcedure("/toqui.v1.TripService/ListTrips") {
		t.Error("expected ListTrips to NOT be classified as AI procedure")
	}
}

func TestIsGeoProcedure(t *testing.T) {
	if !isGeoProcedure("/toqui.v1.LocationService/GetNearby") {
		t.Error("expected GetNearby to be classified as geo procedure")
	}
	if isGeoProcedure("/toqui.v1.ChatService/SendMessage") {
		t.Error("expected SendMessage to NOT be classified as geo procedure")
	}
	if isGeoProcedure("/toqui.v1.LocationService/UpdateLocation") {
		t.Error("expected UpdateLocation to NOT be classified as geo procedure")
	}
}

func TestInterceptor_GeoLimiterSeparateFromGeneral(t *testing.T) {
	// 10 AI/min, 60 general/min — geo is fixed at 30/min.
	i := NewInterceptor(10, 60)
	defer i.Stop()

	// Create a user entry and verify all three limiters are created.
	uid := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	entry := i.getOrCreate(uid)

	if entry.aiLimiter == nil {
		t.Fatal("aiLimiter should not be nil")
	}
	if entry.geoLimiter == nil {
		t.Fatal("geoLimiter should not be nil")
	}
	if entry.generalLimiter == nil {
		t.Fatal("generalLimiter should not be nil")
	}

	// Geo limiter should have burst of 30 (matching geoPerMinute).
	// Exhaust burst and verify it blocks.
	for range 30 {
		if !entry.geoLimiter.Allow() {
			t.Fatal("geoLimiter should allow requests within burst")
		}
	}
	if entry.geoLimiter.Allow() {
		t.Error("geoLimiter should block after burst is exhausted")
	}

	// General limiter should still allow requests.
	if !entry.generalLimiter.Allow() {
		t.Error("generalLimiter should still allow requests after geo burst is exhausted")
	}
}
