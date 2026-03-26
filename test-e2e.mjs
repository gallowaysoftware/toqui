/**
 * Phase 5a: Comprehensive end-to-end test
 *
 * Tests every API call the Expo app makes against the local backend.
 * Uses the same ConnectRPC client libraries as the app.
 */

import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { create } from "@bufbuild/protobuf";

const { TripService, TripStatus } = await import("./src/gen/toqui/v1/trip_pb.ts");
const { ChatService, ChatMode } = await import("./src/gen/toqui/v1/chat_pb.ts");
const { AuthService } = await import("./src/gen/toqui/v1/auth_pb.ts");
const { BookingService, IngestBookingRequestSchema, ListBookingsRequestSchema, BookingType } = await import("./src/gen/toqui/v1/booking_pb.ts");

const API_URL = process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8090";
const TOKEN = process.env.TEST_TOKEN;
if (!TOKEN) { console.error("Set TEST_TOKEN"); process.exit(1); }

const transport = createConnectTransport({
  baseUrl: API_URL,
  fetch: (input, init) =>
    globalThis.fetch(input, {
      ...init,
      headers: { ...Object.fromEntries(new Headers(init?.headers).entries()), Origin: "http://localhost:8081" },
    }),
  interceptors: [(next) => async (req) => {
    req.header.set("Authorization", `Bearer ${TOKEN}`);
    return next(req);
  }],
});

let passed = 0;
let failed = 0;

async function test(name, fn) {
  try {
    await fn();
    console.log(`  ✅ ${name}`);
    passed++;
  } catch (e) {
    console.log(`  ❌ ${name}: ${e.message}`);
    failed++;
  }
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

// ============================================================
console.log("\n🔐 Auth Service");
// ============================================================

const authClient = createClient(AuthService, transport);

await test("GetCurrentUser returns user info", async () => {
  const res = await authClient.getCurrentUser({});
  assert(res.user, "no user returned");
  assert(res.user.id, "no user ID");
  assert(res.user.email, "no email");
  console.log(`     user: ${res.user.name} <${res.user.email}>`);
});

// ============================================================
console.log("\n🗺️  Trip Service");
// ============================================================

const tripClient = createClient(TripService, transport);
let testTripId;

await test("CreateTrip creates a new trip", async () => {
  const res = await tripClient.createTrip({
    title: "E2E Test Trip — " + new Date().toISOString().slice(0, 16),
    description: "Automated test trip for Phase 5a validation",
  });
  assert(res.trip, "no trip returned");
  assert(res.trip.id, "no trip ID");
  testTripId = res.trip.id;
  console.log(`     trip ID: ${testTripId}`);
});

await test("ListTrips includes the new trip", async () => {
  const res = await tripClient.listTrips({ pagination: { pageSize: 50 } });
  assert(res.trips.length > 0, "no trips returned");
  const found = res.trips.find(t => t.id === testTripId);
  assert(found, "new trip not in list");
  console.log(`     total trips: ${res.trips.length}`);
});

await test("GetTrip returns the trip", async () => {
  const res = await tripClient.getTrip({ id: testTripId });
  assert(res.trip, "no trip returned");
  assert(res.trip.title.startsWith("E2E Test Trip"), "wrong title");
});

await test("UpdateTrip changes title", async () => {
  const res = await tripClient.updateTrip({ id: testTripId, title: "Updated E2E Trip" });
  assert(res.trip, "no trip returned");
  assert(res.trip.title === "Updated E2E Trip", `title is "${res.trip.title}"`);
});

await test("UpdateTrip changes status to active", async () => {
  const res = await tripClient.updateTrip({ id: testTripId, status: TripStatus.ACTIVE });
  assert(res.trip, "no trip returned");
  assert(res.trip.status === TripStatus.ACTIVE, `status is ${res.trip.status}`);
});

await test("GetItinerary returns (empty) itinerary", async () => {
  const res = await tripClient.getItinerary({ tripId: testTripId });
  assert(res.itinerary !== undefined, "no itinerary object");
});

// ============================================================
console.log("\n💬 Chat Service (Streaming)");
// ============================================================

const chatClient = createClient(ChatService, transport);

await test("SendMessage streams text deltas", async () => {
  const events = [];
  let fullText = "";
  for await (const event of chatClient.sendMessage({
    sessionId: "",
    tripId: testTripId,
    content: "Say hello in one sentence",
    mode: ChatMode.PLANNING,
  })) {
    events.push(event.event.case);
    if (event.event.case === "textDelta") {
      fullText += event.event.value.text;
    }
  }
  assert(events.includes("sessionCreated"), "no sessionCreated event");
  assert(events.includes("textDelta"), "no textDelta events");
  assert(events.includes("messageComplete"), "no messageComplete event");
  assert(fullText.length > 10, `text too short: "${fullText.slice(0, 50)}"`);
  console.log(`     events: ${events.join(" → ")}`);
  console.log(`     text: "${fullText.slice(0, 80)}..."`);
});

await test("GetChatHistory returns messages", async () => {
  const res = await chatClient.getChatHistory({
    tripId: testTripId,
    sessionId: "",
    pagination: { pageSize: 50, pageToken: "" },
  });
  assert(res.messages.length >= 2, `only ${res.messages.length} messages`);
  console.log(`     messages: ${res.messages.length}`);
});

// ============================================================
console.log("\n📋 Booking Service");
// ============================================================

const bookingClient = createClient(BookingService, transport);
let testBookingId;

await test("IngestBooking creates a booking from raw text", async () => {
  const res = await bookingClient.ingestBooking(
    create(IngestBookingRequestSchema, {
      tripId: testTripId,
      type: BookingType.FLIGHT,
      rawText: "Confirmation: ABC123\nAir Canada Flight AC301\nToronto YYZ → Tokyo NRT\nMar 15, 2026 at 1:30 PM\nPassenger: Test User",
    })
  );
  assert(res.booking, "no booking returned");
  testBookingId = res.booking.id;
  console.log(`     booking ID: ${testBookingId}, title: "${res.booking.title}"`);
});

await test("ListBookings returns the booking", async () => {
  const res = await bookingClient.listBookings(
    create(ListBookingsRequestSchema, {
      tripId: testTripId,
      pagination: { pageSize: 100, pageToken: "" },
    })
  );
  assert(res.bookings.length > 0, "no bookings");
  console.log(`     bookings: ${res.bookings.length}`);
});

await test("DeleteBooking removes it", async () => {
  await bookingClient.deleteBooking({ id: testBookingId });
  const res = await bookingClient.listBookings(
    create(ListBookingsRequestSchema, {
      tripId: testTripId,
      pagination: { pageSize: 100, pageToken: "" },
    })
  );
  const found = res.bookings.find(b => b.id === testBookingId);
  assert(!found, "booking still exists after delete");
});

// ============================================================
console.log("\n🌐 HTTP Endpoints");
// ============================================================

await test("GET /healthz returns healthy", async () => {
  const res = await fetch(`${API_URL}/healthz`);
  assert(res.ok, `status ${res.status}`);
});

await test("GET /api/usage returns usage data", async () => {
  const res = await fetch(`${API_URL}/api/usage`, {
    headers: { Authorization: `Bearer ${TOKEN}`, Origin: "http://localhost:8081" },
  });
  assert(res.ok, `status ${res.status}`);
  const data = await res.json();
  assert(typeof data.used === "number", "no used field");
  console.log(`     used: ${data.used}/${data.limit}`);
});

// ============================================================
// Cleanup
// ============================================================
console.log("\n🧹 Cleanup");

await test("DeleteTrip removes test trip", async () => {
  await tripClient.deleteTrip({ id: testTripId });
  const res = await tripClient.listTrips({ pagination: { pageSize: 50 } });
  const found = res.trips.find(t => t.id === testTripId);
  assert(!found, "trip still exists after delete");
});

// ============================================================
console.log(`\n${"=".repeat(50)}`);
console.log(`  ${passed} passed, ${failed} failed`);
console.log(`${"=".repeat(50)}\n`);

if (failed > 0) process.exit(1);
