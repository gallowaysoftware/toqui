#!/usr/bin/env python3
"""
Alice Solo Backpacker - Toqui Backend Integration Test Runner

Simulates a full plan/travel cycle via the ConnectRPC API.
Uses the Connect protocol (application/json) over HTTP/1.1.

For streaming (SendMessage), uses the Connect streaming protocol:
  POST with application/json, response is newline-delimited JSON envelopes.

Usage:
  python3 feedback/alice-test-runner.py
"""

import hmac
import hashlib
import base64
import json
import http.client
import time
import sys
import os

BASE_URL = "localhost:8090"
USER_ID = "00000000-0000-0000-0000-000000000002"
JWT_SECRET = "dev-secret-change-in-production"

# ── Helpers ──────────────────────────────────────────────────────────────────

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()

def generate_jwt() -> str:
    header = b64url(json.dumps({"alg": "HS256", "typ": "JWT"}).encode())
    now = int(time.time())
    payload = b64url(json.dumps({"sub": USER_ID, "exp": now + 3600, "iat": now}).encode())
    sig = b64url(hmac.new(JWT_SECRET.encode(), f"{header}.{payload}".encode(), hashlib.sha256).digest())
    return f"{header}.{payload}.{sig}"

def connect_rpc_call(path: str, body: dict, token: str) -> dict:
    """Make a unary ConnectRPC call (application/json)."""
    conn = http.client.HTTPConnection(BASE_URL, timeout=30)
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {token}",
        "Connect-Protocol-Version": "1",
    }
    conn.request("POST", path, json.dumps(body), headers)
    resp = conn.getresponse()
    raw = resp.read().decode()
    status = resp.status
    conn.close()

    result = {"status": status, "raw": raw}
    try:
        result["json"] = json.loads(raw)
    except json.JSONDecodeError:
        result["json"] = None
    return result

def connect_rpc_stream(path: str, body: dict, token: str) -> list:
    """
    Make a server-streaming ConnectRPC call.
    Connect protocol streaming: response is newline-delimited JSON envelopes.
    Each envelope: {"result": {...}} for data, or {"error": {...}} for errors.
    """
    conn = http.client.HTTPConnection(BASE_URL, timeout=120)
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {token}",
        "Connect-Protocol-Version": "1",
    }
    conn.request("POST", path, json.dumps(body), headers)
    resp = conn.getresponse()

    events = []
    status = resp.status

    if status != 200:
        raw = resp.read().decode()
        conn.close()
        return [{"type": "error", "status": status, "raw": raw}]

    # Read the streaming response
    # Connect protocol uses chunked transfer encoding with JSON envelopes
    raw_data = resp.read().decode()
    conn.close()

    # Parse each line as a JSON envelope
    for line in raw_data.strip().split("\n"):
        line = line.strip()
        if not line:
            continue
        try:
            envelope = json.loads(line)
            events.append(envelope)
        except json.JSONDecodeError:
            events.append({"type": "raw", "data": line})

    return events

# ── Step Functions ───────────────────────────────────────────────────────────

def step1_selection_mode(token: str) -> dict:
    """Send selection mode message about Vietnam backpacking."""
    print("\n" + "=" * 70)
    print("STEP 1: Selection Mode — Trip Creation")
    print("=" * 70)

    body = {
        "content": "I'm thinking about backpacking through Vietnam for a month. Street food, motorbikes, the whole deal",
        "mode": 3,  # CHAT_MODE_SELECTION
        "tripId": "",
    }

    print(f"  Request: mode=SELECTION, tripId='', content='{body['content'][:60]}...'")
    start = time.time()

    events = connect_rpc_stream("/toqui.v1.ChatService/SendMessage", body, token)

    elapsed = time.time() - start
    print(f"  Elapsed: {elapsed:.2f}s")
    print(f"  Events received: {len(events)}")

    result = {
        "elapsed": elapsed,
        "events": events,
        "session_id": None,
        "trip_id": None,
        "trip_title": None,
        "full_response": "",
        "tool_calls": [],
        "errors": [],
    }

    for evt in events:
        r = evt.get("result", evt)

        # Session created
        if "sessionCreated" in r:
            result["session_id"] = r["sessionCreated"].get("sessionId")
            print(f"  Session created: {result['session_id']}")

        # Text delta
        if "textDelta" in r:
            result["full_response"] += r["textDelta"].get("text", "")

        # Tool call
        if "toolCall" in r:
            tc = r["toolCall"]
            result["tool_calls"].append(tc)
            print(f"  Tool call: {tc.get('toolName')} — {tc.get('inputJson', '')[:100]}")

        # Tool result
        if "toolResult" in r:
            tr = r["toolResult"]
            print(f"  Tool result: {tr.get('toolName')} — {tr.get('resultJson', '')[:100]}")

        # Trip created
        if "tripCreated" in r:
            trip = r["tripCreated"].get("trip", {})
            result["trip_id"] = trip.get("id")
            result["trip_title"] = trip.get("title")
            print(f"  TRIP CREATED: id={result['trip_id']}, title='{result['trip_title']}'")

        # Trip selected
        if "tripSelected" in r:
            trip = r["tripSelected"].get("trip", {})
            result["trip_id"] = trip.get("id")
            result["trip_title"] = trip.get("title")
            print(f"  TRIP SELECTED: id={result['trip_id']}, title='{result['trip_title']}'")

        # Message complete
        if "messageComplete" in r:
            mc = r["messageComplete"]
            result["full_response"] = mc.get("fullContent", result["full_response"])
            result["session_id"] = mc.get("sessionId", result["session_id"])

        # Error
        if "error" in r:
            result["errors"].append(r["error"])
            print(f"  ERROR: {r['error']}")

    print(f"\n  AI Response ({len(result['full_response'])} chars):")
    print(f"  {result['full_response'][:500]}...")

    return result


def step2_planning_route(token: str, trip_id: str, session_id: str) -> dict:
    """Planning mode — route planning."""
    print("\n" + "=" * 70)
    print("STEP 2: Planning Mode — Route Planning")
    print("=" * 70)

    body = {
        "content": "What's the best route? I want to start in Hanoi and end in Ho Chi Minh City",
        "mode": 1,  # CHAT_MODE_PLANNING
        "tripId": trip_id,
        "sessionId": session_id or "",
    }

    print(f"  Request: mode=PLANNING, tripId='{trip_id}', content='{body['content'][:60]}...'")
    start = time.time()

    events = connect_rpc_stream("/toqui.v1.ChatService/SendMessage", body, token)

    elapsed = time.time() - start
    print(f"  Elapsed: {elapsed:.2f}s")
    print(f"  Events received: {len(events)}")

    result = {
        "elapsed": elapsed,
        "events": events,
        "session_id": session_id,
        "full_response": "",
        "tool_calls": [],
        "errors": [],
    }

    for evt in events:
        r = evt.get("result", evt)

        if "sessionCreated" in r:
            result["session_id"] = r["sessionCreated"].get("sessionId")
            print(f"  Session created: {result['session_id']}")

        if "textDelta" in r:
            result["full_response"] += r["textDelta"].get("text", "")

        if "toolCall" in r:
            tc = r["toolCall"]
            result["tool_calls"].append(tc)
            print(f"  Tool call: {tc.get('toolName')} — {tc.get('inputJson', '')[:100]}")

        if "toolResult" in r:
            tr = r["toolResult"]
            print(f"  Tool result: {tr.get('toolName')} — {tr.get('resultJson', '')[:100]}")

        if "messageComplete" in r:
            mc = r["messageComplete"]
            result["full_response"] = mc.get("fullContent", result["full_response"])
            result["session_id"] = mc.get("sessionId", result["session_id"])

        if "error" in r:
            result["errors"].append(r["error"])
            print(f"  ERROR: {r['error']}")

    print(f"\n  AI Response ({len(result['full_response'])} chars):")
    print(f"  {result['full_response'][:500]}...")

    return result


def step3_planning_budget(token: str, trip_id: str, session_id: str) -> dict:
    """Planning mode — budget advice."""
    print("\n" + "=" * 70)
    print("STEP 3: Planning Mode — Budget Advice")
    print("=" * 70)

    body = {
        "content": "I'm on a tight budget, maybe $30/day. What kind of hostels should I look for?",
        "mode": 1,  # CHAT_MODE_PLANNING
        "tripId": trip_id,
        "sessionId": session_id or "",
    }

    print(f"  Request: mode=PLANNING, tripId='{trip_id}', sessionId='{session_id}', content='{body['content'][:60]}...'")
    start = time.time()

    events = connect_rpc_stream("/toqui.v1.ChatService/SendMessage", body, token)

    elapsed = time.time() - start
    print(f"  Elapsed: {elapsed:.2f}s")

    result = {
        "elapsed": elapsed,
        "events": events,
        "session_id": session_id,
        "full_response": "",
        "tool_calls": [],
        "errors": [],
    }

    for evt in events:
        r = evt.get("result", evt)

        if "textDelta" in r:
            result["full_response"] += r["textDelta"].get("text", "")

        if "toolCall" in r:
            tc = r["toolCall"]
            result["tool_calls"].append(tc)
            print(f"  Tool call: {tc.get('toolName')} — {tc.get('inputJson', '')[:100]}")

        if "toolResult" in r:
            tr = r["toolResult"]
            print(f"  Tool result: {tr.get('toolName')} — {tr.get('resultJson', '')[:100]}")

        if "messageComplete" in r:
            mc = r["messageComplete"]
            result["full_response"] = mc.get("fullContent", result["full_response"])

        if "error" in r:
            result["errors"].append(r["error"])
            print(f"  ERROR: {r['error']}")

    print(f"\n  AI Response ({len(result['full_response'])} chars):")
    print(f"  {result['full_response'][:500]}...")

    return result


def step4_start_traveling(token: str, trip_id: str) -> dict:
    """Update trip status to ACTIVE."""
    print("\n" + "=" * 70)
    print("STEP 4: Status Transition — Start Traveling")
    print("=" * 70)

    body = {
        "id": trip_id,
        "status": 2,  # TRIP_STATUS_ACTIVE
    }

    print(f"  Request: UpdateTrip id='{trip_id}', status=TRIP_STATUS_ACTIVE (2)")
    start = time.time()

    resp = connect_rpc_call("/toqui.v1.TripService/UpdateTrip", body, token)

    elapsed = time.time() - start
    print(f"  Elapsed: {elapsed:.2f}s")
    print(f"  Status: {resp['status']}")

    if resp["json"]:
        trip = resp["json"].get("trip", {})
        print(f"  Trip status: {trip.get('status')}")
        print(f"  Trip title: {trip.get('title')}")
    else:
        print(f"  Raw: {resp['raw'][:200]}")

    return {"elapsed": elapsed, "response": resp}


def step5_companion_mode(token: str, trip_id: str) -> dict:
    """Companion mode — on arrival at Hanoi."""
    print("\n" + "=" * 70)
    print("STEP 5: Companion Mode — On Arrival")
    print("=" * 70)

    body = {
        "content": "I just arrived at Hanoi airport. Where should I go first?",
        "mode": 2,  # CHAT_MODE_COMPANION
        "tripId": trip_id,
    }

    print(f"  Request: mode=COMPANION, tripId='{trip_id}', content='{body['content'][:60]}...'")
    start = time.time()

    events = connect_rpc_stream("/toqui.v1.ChatService/SendMessage", body, token)

    elapsed = time.time() - start
    print(f"  Elapsed: {elapsed:.2f}s")

    result = {
        "elapsed": elapsed,
        "events": events,
        "session_id": None,
        "full_response": "",
        "tool_calls": [],
        "errors": [],
    }

    for evt in events:
        r = evt.get("result", evt)

        if "sessionCreated" in r:
            result["session_id"] = r["sessionCreated"].get("sessionId")
            print(f"  Session created: {result['session_id']}")

        if "textDelta" in r:
            result["full_response"] += r["textDelta"].get("text", "")

        if "toolCall" in r:
            tc = r["toolCall"]
            result["tool_calls"].append(tc)
            print(f"  Tool call: {tc.get('toolName')} — {tc.get('inputJson', '')[:100]}")

        if "toolResult" in r:
            tr = r["toolResult"]
            print(f"  Tool result: {tr.get('toolName')} — {tr.get('resultJson', '')[:100]}")

        if "messageComplete" in r:
            mc = r["messageComplete"]
            result["full_response"] = mc.get("fullContent", result["full_response"])
            result["session_id"] = mc.get("sessionId", result["session_id"])

        if "error" in r:
            result["errors"].append(r["error"])
            print(f"  ERROR: {r['error']}")

    print(f"\n  AI Response ({len(result['full_response'])} chars):")
    print(f"  {result['full_response'][:500]}...")

    return result


def step6_complete_trip(token: str, trip_id: str) -> dict:
    """Update trip status to COMPLETED."""
    print("\n" + "=" * 70)
    print("STEP 6: Status Transition — Complete Trip")
    print("=" * 70)

    body = {
        "id": trip_id,
        "status": 3,  # TRIP_STATUS_COMPLETED
    }

    print(f"  Request: UpdateTrip id='{trip_id}', status=TRIP_STATUS_COMPLETED (3)")
    start = time.time()

    resp = connect_rpc_call("/toqui.v1.TripService/UpdateTrip", body, token)

    elapsed = time.time() - start
    print(f"  Elapsed: {elapsed:.2f}s")
    print(f"  Status: {resp['status']}")

    if resp["json"]:
        trip = resp["json"].get("trip", {})
        print(f"  Trip status: {trip.get('status')}")
        print(f"  Trip title: {trip.get('title')}")
    else:
        print(f"  Raw: {resp['raw'][:200]}")

    return {"elapsed": elapsed, "response": resp}


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    print("=" * 70)
    print("ALICE SOLO BACKPACKER — Toqui Backend Integration Test")
    print("=" * 70)

    # Generate JWT
    token = generate_jwt()
    print(f"\nJWT generated for user {USER_ID}")
    print(f"Token: {token[:50]}...")

    # Pre-check: is backend running?
    print("\nChecking backend connectivity...")
    try:
        resp = connect_rpc_call("/toqui.v1.TripService/ListTrips", {}, token)
        print(f"  Backend status: {resp['status']}")
        if resp['status'] != 200:
            print(f"  WARNING: Backend returned {resp['status']}: {resp['raw'][:200]}")
        else:
            trips = resp["json"].get("trips", [])
            print(f"  Existing trips: {len(trips)}")
    except Exception as e:
        print(f"  FATAL: Cannot connect to backend at {BASE_URL}: {e}")
        print("  Make sure the backend is running: go run ./cmd/server")
        sys.exit(1)

    results = {}

    # Step 1: Selection mode
    try:
        results["step1"] = step1_selection_mode(token)
    except Exception as e:
        print(f"  EXCEPTION: {e}")
        results["step1"] = {"error": str(e)}

    # Extract trip_id for subsequent steps
    trip_id = results.get("step1", {}).get("trip_id")
    session_id = results.get("step1", {}).get("session_id")

    if not trip_id:
        print("\n  WARNING: No trip was created in step 1.")
        print("  Attempting to create trip manually via CreateTrip...")
        try:
            resp = connect_rpc_call("/toqui.v1.TripService/CreateTrip", {
                "title": "Vietnam Backpacking",
                "description": "Month-long backpacking trip through Vietnam. Street food, motorbikes, the whole deal.",
            }, token)
            if resp["json"] and resp["json"].get("trip"):
                trip_id = resp["json"]["trip"]["id"]
                print(f"  Manually created trip: {trip_id}")
            else:
                print(f"  Failed to create trip: {resp['raw'][:200]}")
        except Exception as e:
            print(f"  EXCEPTION creating trip: {e}")

    if not trip_id:
        print("\nFATAL: No trip_id available. Cannot continue.")
        write_results(results, trip_id, session_id)
        sys.exit(1)

    # Step 2: Planning mode — route planning
    try:
        results["step2"] = step2_planning_route(token, trip_id, "")
        if results["step2"].get("session_id"):
            session_id = results["step2"]["session_id"]
    except Exception as e:
        print(f"  EXCEPTION: {e}")
        results["step2"] = {"error": str(e)}

    # Step 3: Planning mode — budget advice (same session)
    try:
        results["step3"] = step3_planning_budget(token, trip_id, session_id or "")
    except Exception as e:
        print(f"  EXCEPTION: {e}")
        results["step3"] = {"error": str(e)}

    # Step 4: Update trip to ACTIVE
    try:
        results["step4"] = step4_start_traveling(token, trip_id)
    except Exception as e:
        print(f"  EXCEPTION: {e}")
        results["step4"] = {"error": str(e)}

    # Step 5: Companion mode
    try:
        results["step5"] = step5_companion_mode(token, trip_id)
    except Exception as e:
        print(f"  EXCEPTION: {e}")
        results["step5"] = {"error": str(e)}

    # Step 6: Complete trip
    try:
        results["step6"] = step6_complete_trip(token, trip_id)
    except Exception as e:
        print(f"  EXCEPTION: {e}")
        results["step6"] = {"error": str(e)}

    # Write results to JSON for post-processing
    write_results(results, trip_id, session_id)

    print("\n" + "=" * 70)
    print("TEST COMPLETE")
    print("=" * 70)


def write_results(results: dict, trip_id: str, session_id: str):
    """Write raw results to JSON for the feedback document generator."""
    output = {
        "trip_id": trip_id,
        "session_id": session_id,
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "results": {}
    }

    for key, val in results.items():
        # Strip large event arrays but keep summaries
        if isinstance(val, dict):
            clean = {}
            for k, v in val.items():
                if k == "events":
                    clean[k] = f"[{len(v)} events]"
                else:
                    clean[k] = v
            output["results"][key] = clean
        else:
            output["results"][key] = val

    out_path = os.path.join(os.path.dirname(__file__), "alice-test-results.json")
    with open(out_path, "w") as f:
        json.dump(output, f, indent=2, default=str)
    print(f"\nResults written to {out_path}")


if __name__ == "__main__":
    main()
