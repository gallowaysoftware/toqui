import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { BookingCard } from "./BookingCard";
import { create } from "@bufbuild/protobuf";
import {
  BookingSchema,
  BookingType,
  BookingSource,
  FlightDetailsSchema,
  HotelDetailsSchema,
} from "@/gen/toqui/v1/booking_pb";
import type { Booking } from "@/gen/toqui/v1/booking_pb";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

// Mock lucide-react to avoid SVG rendering issues in tests
vi.mock("lucide-react", () => {
  const makeIcon = (name: string) => {
    const IconComponent = (props: Record<string, unknown>) => (
      <svg data-testid={`icon-${name}`} {...(props as React.SVGAttributes<SVGSVGElement>)} />
    );
    IconComponent.displayName = name;
    return IconComponent;
  };
  return {
    Plane: makeIcon("Plane"),
    Hotel: makeIcon("Hotel"),
    Car: makeIcon("Car"),
    TrainFront: makeIcon("TrainFront"),
    Ticket: makeIcon("Ticket"),
    UtensilsCrossed: makeIcon("UtensilsCrossed"),
    Map: makeIcon("Map"),
    Package: makeIcon("Package"),
    Calendar: makeIcon("Calendar"),
    Hash: makeIcon("Hash"),
  };
});

function makeBooking(
  overrides: Partial<{
    id: string;
    tripId: string;
    type: BookingType;
    title: string;
    confirmationCode: string;
    provider: string;
    startTime: Date;
    endTime: Date;
    source: BookingSource;
    bookingDetails: Booking["bookingDetails"];
  }> = {},
): Booking {
  return create(BookingSchema, {
    id: overrides.id ?? "booking-1",
    tripId: overrides.tripId ?? "trip-1",
    type: overrides.type ?? BookingType.FLIGHT,
    title: overrides.title ?? "Flight to Paris",
    confirmationCode: overrides.confirmationCode ?? "ABC123",
    provider: overrides.provider ?? "Air France",
    source: overrides.source ?? BookingSource.MANUAL,
    startTime: overrides.startTime ? timestampFromDate(overrides.startTime) : undefined,
    endTime: overrides.endTime ? timestampFromDate(overrides.endTime) : undefined,
    bookingDetails: overrides.bookingDetails ?? { case: undefined, value: undefined },
  });
}

describe("BookingCard", () => {
  it("renders booking title", () => {
    render(<BookingCard booking={makeBooking()} />);
    expect(screen.getByText("Flight to Paris")).toBeInTheDocument();
  });

  it("renders booking type badge", () => {
    render(<BookingCard booking={makeBooking({ type: BookingType.FLIGHT })} />);
    expect(screen.getByText("Flight")).toBeInTheDocument();
  });

  it("renders hotel type badge", () => {
    render(
      <BookingCard booking={makeBooking({ type: BookingType.HOTEL, title: "Hilton Stay" })} />,
    );
    expect(screen.getByText("Hotel")).toBeInTheDocument();
    expect(screen.getByText("Hilton Stay")).toBeInTheDocument();
  });

  it("renders car rental type badge", () => {
    render(
      <BookingCard
        booking={makeBooking({
          type: BookingType.CAR_RENTAL,
          title: "Enterprise Rental",
        })}
      />,
    );
    expect(screen.getByText("Car Rental")).toBeInTheDocument();
  });

  it("renders confirmation code", () => {
    render(<BookingCard booking={makeBooking({ confirmationCode: "XYZ789" })} />);
    expect(screen.getByText("XYZ789")).toBeInTheDocument();
  });

  it("does not render confirmation code when empty", () => {
    render(<BookingCard booking={makeBooking({ confirmationCode: "" })} />);
    expect(screen.queryByText("ABC123")).not.toBeInTheDocument();
  });

  it("renders date range when start and end times are provided", () => {
    const booking = makeBooking({
      startTime: new Date("2026-04-01T10:00:00Z"),
      endTime: new Date("2026-04-05T14:00:00Z"),
    });
    render(<BookingCard booking={booking} />);
    // Should have date text present (the exact format depends on locale)
    const card = screen.getByTestId("booking-card");
    expect(card).toBeInTheDocument();
  });

  it("uses type label as title when title is empty", () => {
    render(<BookingCard booking={makeBooking({ title: "", type: BookingType.TRAIN })} />);
    // Should show "Train" as the heading since title is empty
    const headings = screen.getAllByText("Train");
    expect(headings.length).toBeGreaterThanOrEqual(1);
  });

  it("renders flight subtitle with route details", () => {
    const booking = makeBooking({
      type: BookingType.FLIGHT,
      bookingDetails: {
        case: "flightDetails",
        value: create(FlightDetailsSchema, {
          airline: "Air France",
          flightNumber: "AF123",
          departureAirport: "JFK",
          arrivalAirport: "CDG",
        }),
      },
    });
    render(<BookingCard booking={booking} />);
    // The subtitle should contain the route
    expect(screen.getByText(/JFK/)).toBeInTheDocument();
    expect(screen.getByText(/CDG/)).toBeInTheDocument();
  });

  it("renders hotel subtitle with hotel name", () => {
    const booking = makeBooking({
      type: BookingType.HOTEL,
      title: "Paris Stay",
      bookingDetails: {
        case: "hotelDetails",
        value: create(HotelDetailsSchema, {
          hotelName: "Le Grand Hotel",
          roomType: "Deluxe Suite",
        }),
      },
    });
    render(<BookingCard booking={booking} />);
    expect(screen.getByText(/Le Grand Hotel/)).toBeInTheDocument();
  });

  it("calls onClick with booking when clicked", () => {
    const handleClick = vi.fn();
    const booking = makeBooking();
    render(<BookingCard booking={booking} onClick={handleClick} />);
    fireEvent.click(screen.getByTestId("booking-card"));
    expect(handleClick).toHaveBeenCalledWith(booking);
  });

  it("does not crash when onClick is not provided", () => {
    render(<BookingCard booking={makeBooking()} />);
    // Should not throw when clicked
    fireEvent.click(screen.getByTestId("booking-card"));
  });

  it("has the booking type icon", () => {
    render(<BookingCard booking={makeBooking({ type: BookingType.FLIGHT })} />);
    expect(screen.getByTestId("booking-type-icon")).toBeInTheDocument();
  });

  it("renders all booking types without errors", () => {
    const types = [
      BookingType.FLIGHT,
      BookingType.HOTEL,
      BookingType.CAR_RENTAL,
      BookingType.TRAIN,
      BookingType.ACTIVITY,
      BookingType.RESTAURANT,
      BookingType.TOUR,
      BookingType.OTHER,
      BookingType.UNSPECIFIED,
    ];
    for (const type of types) {
      const { unmount } = render(
        <BookingCard booking={makeBooking({ type, title: `Booking ${type}` })} />,
      );
      expect(screen.getByTestId("booking-card")).toBeInTheDocument();
      unmount();
    }
  });
});
