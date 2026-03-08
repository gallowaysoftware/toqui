import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { TripCard } from "./TripCard";
import { create } from "@bufbuild/protobuf";
import { TripSchema, TripStatus } from "@/gen/toqui/v1/trip_pb";

// Mock lucide-react Calendar icon to avoid SVG rendering issues
vi.mock("lucide-react", () => ({
  Calendar: (props: Record<string, unknown>) => <svg data-testid="calendar-icon" {...props} />,
}));

function makeTrip(
  overrides: Partial<{
    id: string;
    title: string;
    description: string;
    status: TripStatus;
    startDate: string;
    endDate: string;
  }> = {},
) {
  return create(TripSchema, {
    id: overrides.id ?? "trip-1",
    title: overrides.title ?? "Tokyo Adventure",
    description: overrides.description ?? "Two weeks exploring Japan",
    status: overrides.status ?? TripStatus.PLANNING,
    startDate: overrides.startDate ?? "",
    endDate: overrides.endDate ?? "",
  });
}

describe("TripCard", () => {
  it("renders trip title", () => {
    render(<TripCard trip={makeTrip()} />);
    expect(screen.getByText("Tokyo Adventure")).toBeInTheDocument();
  });

  it("renders trip description", () => {
    render(<TripCard trip={makeTrip()} />);
    expect(screen.getByText("Two weeks exploring Japan")).toBeInTheDocument();
  });

  it("renders planning status badge", () => {
    render(<TripCard trip={makeTrip({ status: TripStatus.PLANNING })} />);
    expect(screen.getByText("planning")).toBeInTheDocument();
  });

  it("renders active status badge", () => {
    render(<TripCard trip={makeTrip({ status: TripStatus.ACTIVE })} />);
    expect(screen.getByText("active")).toBeInTheDocument();
  });

  it("renders completed status badge", () => {
    render(<TripCard trip={makeTrip({ status: TripStatus.COMPLETED })} />);
    expect(screen.getByText("completed")).toBeInTheDocument();
  });

  it("links to the correct trip page", () => {
    render(<TripCard trip={makeTrip({ id: "trip-42" })} />);
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/trips/trip-42");
  });

  it("shows date range when start and end dates are provided", () => {
    render(<TripCard trip={makeTrip({ startDate: "2026-04-01", endDate: "2026-04-15" })} />);
    expect(screen.getByText("2026-04-01 - 2026-04-15")).toBeInTheDocument();
  });

  it("shows only start date when no end date", () => {
    render(<TripCard trip={makeTrip({ startDate: "2026-04-01", endDate: "" })} />);
    expect(screen.getByText("2026-04-01")).toBeInTheDocument();
  });

  it("does not show dates section when no dates provided", () => {
    render(<TripCard trip={makeTrip({ startDate: "", endDate: "" })} />);
    expect(screen.queryByTestId("calendar-icon")).not.toBeInTheDocument();
  });

  it("does not render description when empty", () => {
    render(<TripCard trip={makeTrip({ description: "" })} />);
    // Should only have title and status badge, no description paragraph
    expect(screen.queryByText("Two weeks exploring Japan")).not.toBeInTheDocument();
  });
});
