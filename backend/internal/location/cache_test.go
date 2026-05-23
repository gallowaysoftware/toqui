package location

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCache_SetAndGet(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	userID := uuid.New()

	// Initially empty
	if entry := cache.Get(userID); entry != nil {
		t.Fatal("expected nil for missing user")
	}

	// Set a location
	cache.Set(userID, 35.6762, 139.6503, 10.0)

	entry := cache.Get(userID)
	if entry == nil {
		t.Fatal("expected non-nil entry after Set")
	}
	if entry.Lat != 35.6762 {
		t.Errorf("expected lat 35.6762, got %f", entry.Lat)
	}
	if entry.Lng != 139.6503 {
		t.Errorf("expected lng 139.6503, got %f", entry.Lng)
	}
	if entry.Accuracy != 10.0 {
		t.Errorf("expected accuracy 10.0, got %f", entry.Accuracy)
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestCache_Overwrite(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	userID := uuid.New()

	cache.Set(userID, 1.0, 2.0, 5.0)
	cache.Set(userID, 3.0, 4.0, 15.0)

	entry := cache.Get(userID)
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Lat != 3.0 || entry.Lng != 4.0 {
		t.Errorf("expected updated coords (3.0, 4.0), got (%f, %f)", entry.Lat, entry.Lng)
	}
	if entry.Accuracy != 15.0 {
		t.Errorf("expected accuracy 15.0, got %f", entry.Accuracy)
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	userID := uuid.New()

	// Insert an entry with a timestamp 31 minutes ago
	cache.SetWithTime(userID, 48.8566, 2.3522, 20.0, time.Now().Add(-31*time.Minute))

	// Should be considered stale
	if entry := cache.Get(userID); entry != nil {
		t.Error("expected nil for stale entry")
	}
}

func TestCache_TTLFresh(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	userID := uuid.New()

	// Insert an entry 29 minutes ago — still fresh
	cache.SetWithTime(userID, 48.8566, 2.3522, 20.0, time.Now().Add(-29*time.Minute))

	entry := cache.Get(userID)
	if entry == nil {
		t.Fatal("expected non-nil for fresh entry")
	}
	if entry.Lat != 48.8566 {
		t.Errorf("expected lat 48.8566, got %f", entry.Lat)
	}
}

func TestCache_Delete(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	userID := uuid.New()

	cache.Set(userID, 1.0, 2.0, 5.0)
	cache.Delete(userID)

	if entry := cache.Get(userID); entry != nil {
		t.Error("expected nil after Delete")
	}
}

func TestCache_DeleteNonexistent(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	// Should not panic
	cache.Delete(uuid.New())
}

func TestCache_MultipleUsers(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	user1 := uuid.New()
	user2 := uuid.New()

	cache.Set(user1, 10.0, 20.0, 5.0)
	cache.Set(user2, 30.0, 40.0, 10.0)

	e1 := cache.Get(user1)
	e2 := cache.Get(user2)

	if e1 == nil || e2 == nil {
		t.Fatal("expected non-nil entries for both users")
	}
	if e1.Lat != 10.0 || e1.Lng != 20.0 {
		t.Errorf("user1: expected (10.0, 20.0), got (%f, %f)", e1.Lat, e1.Lng)
	}
	if e2.Lat != 30.0 || e2.Lng != 40.0 {
		t.Errorf("user2: expected (30.0, 40.0), got (%f, %f)", e2.Lat, e2.Lng)
	}
}

func TestCache_Len(t *testing.T) {
	cache := NewCache(30 * time.Minute)

	if cache.Len() != 0 {
		t.Errorf("expected len 0, got %d", cache.Len())
	}

	cache.Set(uuid.New(), 1, 2, 3)
	cache.Set(uuid.New(), 4, 5, 6)

	if cache.Len() != 2 {
		t.Errorf("expected len 2, got %d", cache.Len())
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache := NewCache(30 * time.Minute)
	userID := uuid.New()

	var wg sync.WaitGroup
	// Run concurrent reads and writes
	for i := range 100 {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			cache.Set(userID, float64(n), float64(n*2), float64(n))
		}(i)
		go func() {
			defer wg.Done()
			_ = cache.Get(userID)
		}()
	}
	wg.Wait()

	// After all goroutines finish, should have exactly one entry
	if cache.Len() != 1 {
		t.Errorf("expected len 1, got %d", cache.Len())
	}
}
