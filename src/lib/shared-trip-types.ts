/**
 * Types for the public shared trip endpoint: GET /shared/{token}
 * These are plain JSON types (not protobuf) since the endpoint returns raw JSON.
 */

export interface SharedTripInfo {
  title: string;
  description?: string;
  destination_country?: string;
  status: string;
  start_date?: string;
  end_date?: string;
}

export interface SharedItineraryItem {
  title: string;
  type?: string;
  description?: string;
}

export interface SharedItineraryDay {
  day_number: number;
  items: SharedItineraryItem[];
}

export interface SharedTripResponse {
  trip: SharedTripInfo;
  itinerary: SharedItineraryDay[];
}
