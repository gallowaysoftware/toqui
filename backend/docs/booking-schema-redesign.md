# Booking Schema Redesign

## 1. Overview

The booking schema redesign introduces **typed booking details** to replace the previous unstructured `details_json` blob. The changes span the database layer, protobuf definitions, Go domain types, AI prompt design, and handler mapping.

**Goals:**

- Provide strongly-typed, per-booking-type detail structs (flight, hotel, car rental, train, tour, activity, restaurant) so the frontend can render type-specific UI without parsing raw JSON.
- Promote commonly-queried fields (`departure_location`, `arrival_location`, `num_guests`) to first-class SQL columns for direct querying and indexing.
- Preserve the raw JSONB `details_json` column as the storage layer for type-specific detail blobs, while exposing them as a proto `oneof` on the wire.
- Add an `ExtractBookingField` RPC that re-extracts information from the stored raw source text when the initial parse missed a field.

**What did NOT change:** The `raw_source` column, the `source` enum, pagination, auth model, and all existing RPCs remain untouched. Existing rows with only `details_json` populated continue to work.

---

## 2. Database Changes

### Migration: `20260305000004_booking_details`

**Up migration** (`20260305000004_booking_details.up.sql`):

```sql
ALTER TABLE bookings ADD COLUMN departure_location TEXT;
ALTER TABLE bookings ADD COLUMN arrival_location TEXT;
ALTER TABLE bookings ADD COLUMN num_guests INT;
```

**Down migration** (`20260305000004_booking_details.down.sql`):

```sql
ALTER TABLE bookings DROP COLUMN IF EXISTS departure_location;
ALTER TABLE bookings DROP COLUMN IF EXISTS arrival_location;
ALTER TABLE bookings DROP COLUMN IF EXISTS num_guests;
```

All three columns are nullable, so the migration is additive and safe to run on a live database with existing rows.

### What lives where

| Storage                     | Contents                                        | Purpose                                                                   |
| --------------------------- | ----------------------------------------------- | ------------------------------------------------------------------------- |
| `departure_location` (TEXT) | Origin city, airport code, or station name      | Queryable top-level field for flights, trains, tours                      |
| `arrival_location` (TEXT)   | Destination city, airport code, or station name | Queryable top-level field for flights, trains, tours                      |
| `num_guests` (INT)          | Guest/passenger count                           | Queryable top-level field for hotels, activities, restaurants             |
| `details_json` (JSONB)      | Full type-specific detail object                | Stores all type-specific fields (airline, flight number, room type, etc.) |
| `raw_source` (TEXT)         | Original pasted/emailed text                    | Preserved for re-extraction via `ExtractBookingField`                     |

**Design rationale:** Fields that are useful for filtering, sorting, or quick display are promoted to SQL columns. Deeply nested, type-specific fields (seat assignments, terminal info, tour stops) remain in the JSONB blob to avoid schema explosion.

### Updated queries (`bookings.sql`)

`CreateBooking` now accepts the three new columns as parameters (`$14`, `$15`, `$16`):

```sql
INSERT INTO bookings (user_id, trip_id, type, confirmation_code, provider, title,
  start_time, end_time, location, address, details_json, raw_source, source,
  departure_location, arrival_location, num_guests)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;
```

All other queries (`GetBookingByID`, `ListBookingsByTrip`, `ListBookingsByUser`, `LinkBookingToTrip`, `DeleteBooking`) use `SELECT *` or don't touch detail columns, so they automatically pick up the new fields after sqlc regeneration.

---

## 3. Proto Changes

### File: `proto/toqui/v1/booking.proto`

### New type-specific message types

Seven detail messages were added, each matching a booking type:

| Message             | Key fields                                                                                    |
| ------------------- | --------------------------------------------------------------------------------------------- |
| `FlightDetails`     | airline, flight_number, departure/arrival_airport, terminals, seat, cabin_class, passengers[] |
| `HotelDetails`      | hotel_name, check_in/out_date, room_type, num_guests, address, phone                          |
| `CarRentalDetails`  | company, pickup/dropoff_location, pickup/dropoff_time, car_type, driver_name                  |
| `TrainDetails`      | operator, train_number, departure/arrival_station, seat, car_number, class                    |
| `TourDetails`       | tour_operator, tour_name, num_participants, meeting_point, stops[] (TourStop)                 |
| `ActivityDetails`   | operator, activity_name, location, num_guests, notes                                          |
| `RestaurantDetails` | restaurant_name, cuisine, party_size, notes                                                   |

`TourStop` is a nested message used by `TourDetails.stops`.

### Booking message changes

New scalar fields on `Booking`:

- `string departure_location = 15`
- `string arrival_location = 16`
- `int32 num_guests = 17`

New `oneof booking_details` (field numbers 20-26):

```protobuf
oneof booking_details {
  FlightDetails flight_details = 20;
  HotelDetails hotel_details = 21;
  CarRentalDetails car_rental_details = 22;
  TrainDetails train_details = 23;
  TourDetails tour_details = 24;
  ActivityDetails activity_details = 25;
  RestaurantDetails restaurant_details = 26;
}
```

### Deprecated field

`string details_json = 11 [deprecated = true]` -- the raw JSON string is still populated for backward compatibility but clients should migrate to the `oneof`.

### New RPC

```protobuf
rpc ExtractBookingField(ExtractBookingFieldRequest) returns (ExtractBookingFieldResponse);
```

- **Request:** `booking_id` (UUID) + `question` (string, 1-1000 chars)
- **Response:** `answer` (string) + `extracted_fields` (map<string, string>)

### New enums

`BOOKING_TYPE_TOUR = 8` was added to `BookingType`.

---

## 4. Go Struct Mapping (`internal/booking/details.go`)

Seven Go structs mirror the proto detail messages exactly, with JSON tags matching the AI output schema:

- `FlightDetails` -- includes `Passengers []string`
- `HotelDetails` -- includes `NumGuests int`
- `CarRentalDetails`
- `TrainDetails`
- `TourStop` / `TourDetails` -- `Stops []TourStop`
- `ActivityDetails`
- `RestaurantDetails`

### `UnmarshalDetails(bookingType string, raw json.RawMessage) (any, error)`

This function provides type-aware deserialization of the JSONB `details_json` column:

1. Returns `nil, nil` for empty/null/`{}` payloads (no error).
2. Switches on `bookingType` string (`"flight"`, `"hotel"`, etc.) to select the correct target struct.
3. For unknown types, falls back to `map[string]any` generic deserialization.
4. Returns a pointer to the typed struct on success.

This function is the canonical way to go from `(type, json.RawMessage)` to a concrete Go type. It is used internally; the handler layer calls `setBookingDetailsOneof` which performs its own inline unmarshaling (see section 7).

---

## 5. AI Prompt Design

### Type-aware parsing (`service.go` -- `parseWithAI`)

The system prompt instructs the AI to return a JSON object with both top-level fields and a nested `details` object whose schema depends on `type`. The prompt explicitly lists the expected schema for each booking type:

```
flight: {"airline":"","flight_number":"","departure_airport":"",...}
hotel: {"hotel_name":"","check_in_date":"","check_out_date":"",...}
...
```

This ensures the AI produces structured output that matches the Go detail structs exactly.

### Type hints

When the caller provides a `BookingType` in `IngestBookingRequest`, it is forwarded as a type hint prepended to the user message:

```
Type hint: BOOKING_TYPE_FLIGHT

<raw booking text>
```

This helps the AI resolve ambiguous confirmations (e.g., a combined flight+hotel itinerary) toward the intended type.

### `ParsedBooking` struct

The AI response is unmarshaled into:

```go
type ParsedBooking struct {
    Type              string          `json:"type"`
    ConfirmationCode  string          `json:"confirmation_code"`
    Provider          string          `json:"provider"`
    Title             string          `json:"title"`
    StartTime         string          `json:"start_time"`
    EndTime           string          `json:"end_time"`
    Address           string          `json:"address"`
    DepartureLocation string          `json:"departure_location"`
    ArrivalLocation   string          `json:"arrival_location"`
    NumGuests         int32           `json:"num_guests"`
    Details           json.RawMessage `json:"details"`
}
```

The `Details` field is kept as `json.RawMessage` so it can be stored directly into the `details_json` JSONB column without re-serialization. The three promoted fields (`DepartureLocation`, `ArrivalLocation`, `NumGuests`) are extracted at the top level and written to their respective SQL columns.

---

## 6. ExtractField Fallback Re-extraction Flow

The `ExtractBookingField` RPC enables on-demand re-extraction from the original raw source when the initial AI parse missed information. The flow:

1. **Client sends** `ExtractBookingFieldRequest` with a `booking_id` and a natural-language `question` (e.g., "What is my seat number?").

2. **Service loads the booking** from the database and retrieves `raw_source`. If no raw source is stored, the call fails with an error.

3. **AI re-extraction:** The service sends the raw source text and the question to the AI with a dedicated system prompt:
   - The AI answers the question directly (`answer` field).
   - The AI also returns any structured fields it discovers while answering (`extracted_fields` map).

4. **Response returned** to the client with both the answer and the extracted fields map. The client (or a future server-side step) can use `extracted_fields` to patch the booking's `details_json` or top-level columns.

**Key design points:**

- The raw source is never discarded, enabling re-extraction at any time.
- Temperature is set to 0 for deterministic extraction.
- `MaxTokens` is capped at 1024 (vs 2048 for initial ingestion) since extraction answers are shorter.
- The extracted_fields map uses string keys and values, allowing the client to decide how to apply updates.

---

## 7. Handler Changes (`internal/handlers/booking.go`)

### `bookingToProto` mapping

The `bookingToProto` function maps `dbgen.Booking` to `*toquiv1.Booking`. The redesign adds:

- Mapping the three new nullable columns (`DepartureLocation`, `ArrivalLocation`, `NumGuests`) with `.Valid` guards.
- Populating the deprecated `DetailsJson` string field for backward compatibility.
- Calling `setBookingDetailsOneof` to populate the typed `oneof`.

### `setBookingDetailsOneof`

This function switches on the `bookingType` string and unmarshals the raw JSONB into the appropriate Go detail struct from `internal/booking/details.go`, then maps it to the corresponding proto oneof variant:

| DB type string | Proto oneof field           |
| -------------- | --------------------------- |
| `"flight"`     | `Booking_FlightDetails`     |
| `"hotel"`      | `Booking_HotelDetails`      |
| `"car_rental"` | `Booking_CarRentalDetails`  |
| `"train"`      | `Booking_TrainDetails`      |
| `"tour"`       | `Booking_TourDetails`       |
| `"activity"`   | `Booking_ActivityDetails`   |
| `"restaurant"` | `Booking_RestaurantDetails` |

Unmarshal errors are silently swallowed (the `oneof` is simply not set), which is a deliberate choice: a corrupt JSONB blob should not prevent the booking from being returned. The deprecated `details_json` string is still available as a fallback.

For `TourDetails`, the handler performs an extra mapping step to convert `[]booking.TourStop` to `[]*toquiv1.TourStop`.

### New handler: `ExtractBookingField`

Follows the standard pattern: extract `userID` from auth context, parse `booking_id`, delegate to `bookingSvc.ExtractField`, and map the result to `ExtractBookingFieldResponse`.

---

## 8. Migration Notes

### Backward compatibility

- **Additive-only migration:** The three new columns are nullable with no defaults, so `ALTER TABLE ... ADD COLUMN` does not rewrite existing rows. Existing data is completely unaffected.
- **No NOT NULL constraints:** Old rows will have `NULL` for `departure_location`, `arrival_location`, and `num_guests`. The handler's `.Valid` checks handle this gracefully.
- **Proto field numbering:** New fields use numbers 15-17 and 20-26, which do not conflict with any existing field numbers. Wire-compatible with older clients.
- **Deprecated `details_json`:** Still populated on every response. Clients using the old string field will continue to work until they migrate to the `oneof`.
- **`SELECT *` queries:** `GetBookingByID`, `ListBookingsByTrip`, and `ListBookingsByUser` all use `SELECT *`, so they automatically return the new columns after sqlc regeneration without query changes.

### Rollback

The down migration safely drops the three columns with `IF EXISTS`. The `oneof` fields will simply be empty on the wire if the columns don't exist (sqlc would need to be regenerated to match).

### Required steps after applying

1. Run the up migration: `migrate -path db/migrations -database $DB_URL up`
2. Regenerate sqlc: `sqlc generate`
3. Regenerate protos: `make proto-sync`
4. Rebuild and deploy the backend
