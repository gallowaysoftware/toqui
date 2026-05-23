package handlers

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/trip"
)

// PDFExportHandler handles PDF export of trip itineraries.
type PDFExportHandler struct {
	tripSvc *trip.Service
	authSvc *auth.Service
	queries *dbgen.Queries
}

// NewPDFExportHandler creates a new PDFExportHandler.
func NewPDFExportHandler(tripSvc *trip.Service, authSvc *auth.Service, pool *pgxpool.Pool) *PDFExportHandler {
	return &PDFExportHandler{
		tripSvc: tripSvc,
		authSvc: authSvc,
		queries: dbgen.New(pool),
	}
}

// HandleExportPDF handles GET /api/trips/{id}/export/pdf.
func (h *PDFExportHandler) HandleExportPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tripID, ok := parseTripIDFromExportPath(r.URL.Path)
	if !ok {
		http.Error(w, "invalid trip ID", http.StatusBadRequest)
		return
	}

	t, err := h.tripSvc.GetByID(r.Context(), userID, tripID)
	if err != nil {
		slog.Warn("pdf export: trip not found", "trip_id", tripID, "error", err)
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}

	items, err := h.tripSvc.GetItinerary(r.Context(), tripID)
	if err != nil {
		slog.Error("pdf export: failed to load itinerary", "trip_id", tripID, "error", err)
		http.Error(w, "failed to load itinerary", http.StatusInternalServerError)
		return
	}

	bookings, err := h.queries.ListBookingsByTrip(r.Context(), dbgen.ListBookingsByTripParams{
		TripID: pgtype.UUID{Bytes: tripID, Valid: true},
		UserID: userID,
	})
	if err != nil {
		slog.Error("pdf export: failed to load bookings", "trip_id", tripID, "error", err)
		http.Error(w, "failed to load bookings", http.StatusInternalServerError)
		return
	}

	pdfBytes, err := GeneratePDF(t, items, bookings)
	if err != nil {
		slog.Error("pdf export: generation failed", "trip_id", tripID, "error", err)
		http.Error(w, "failed to generate PDF", http.StatusInternalServerError)
		return
	}

	filename := sanitizeFilename(t.Title)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="trip-%s.pdf"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	_, _ = w.Write(pdfBytes)
}

// GeneratePDF creates a formatted PDF for a trip itinerary.
func GeneratePDF(t *dbgen.Trip, items []dbgen.ItineraryItem, bookings []dbgen.Booking) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	// --- Header ---
	pdf.SetFont("Helvetica", "B", 20)
	pdf.Cell(0, 12, safeString(t.Title))
	pdf.Ln(14)

	// Trip metadata
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 100)

	var meta []string
	if t.StartDate.Valid && t.EndDate.Valid {
		meta = append(meta, fmt.Sprintf("%s to %s",
			t.StartDate.Time.Format("Jan 2, 2006"),
			t.EndDate.Time.Format("Jan 2, 2006")))
	}
	if len(t.DestinationCountries) > 0 {
		meta = append(meta, strings.Join(t.DestinationCountries, ", "))
	}
	if t.Status != "" {
		meta = append(meta, fmt.Sprintf("Status: %s", t.Status))
	}
	if len(meta) > 0 {
		pdf.Cell(0, 6, strings.Join(meta, "  |  "))
		pdf.Ln(8)
	}

	if t.Description.Valid && t.Description.String != "" {
		pdf.SetTextColor(80, 80, 80)
		pdf.MultiCell(0, 5, safeString(t.Description.String), "", "", false)
		pdf.Ln(4)
	}

	pdf.SetTextColor(0, 0, 0)

	// --- Horizontal rule ---
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
	pdf.Ln(6)

	// --- Itinerary by day ---
	if len(items) > 0 {
		pdf.SetFont("Helvetica", "B", 14)
		pdf.Cell(0, 8, "Itinerary")
		pdf.Ln(10)

		// Group items by day
		dayItems := make(map[int32][]dbgen.ItineraryItem)
		for _, item := range items {
			day := int32(0)
			if item.DayNumber.Valid {
				day = item.DayNumber.Int32
			}
			dayItems[day] = append(dayItems[day], item)
		}

		// Sort days
		days := make([]int32, 0, len(dayItems))
		for d := range dayItems {
			days = append(days, d)
		}
		sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })

		for _, day := range days {
			dayLabel := "Unscheduled"
			if day > 0 {
				dayLabel = fmt.Sprintf("Day %d", day)
			}

			pdf.SetFont("Helvetica", "B", 12)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(0, 7, dayLabel, "", 1, "", true, 0, "")
			pdf.Ln(2)

			for _, item := range dayItems[day] {
				pdf.SetFont("Helvetica", "B", 10)
				title := ""
				if item.Title.Valid {
					title = item.Title.String
				}

				// Type badge + title
				typeStr := ""
				if item.Type.Valid && item.Type.String != "" {
					typeStr = fmt.Sprintf("[%s] ", item.Type.String)
				}
				pdf.Cell(0, 5, safeString(typeStr+title))
				pdf.Ln(5)

				if item.Description.Valid && item.Description.String != "" {
					pdf.SetFont("Helvetica", "", 9)
					pdf.SetTextColor(80, 80, 80)
					pdf.MultiCell(0, 4, safeString(item.Description.String), "", "", false)
					pdf.SetTextColor(0, 0, 0)
				}

				// Time info
				if item.StartTime.Valid {
					pdf.SetFont("Helvetica", "I", 8)
					pdf.SetTextColor(120, 120, 120)
					timeStr := item.StartTime.Time.Format("3:04 PM")
					if item.EndTime.Valid {
						timeStr += " - " + item.EndTime.Time.Format("3:04 PM")
					}
					pdf.Cell(0, 4, timeStr)
					pdf.Ln(4)
					pdf.SetTextColor(0, 0, 0)
				}

				// Estimated cost
				if item.EstimatedCostCents.Valid && item.EstimatedCostCents.Int64 > 0 {
					pdf.SetFont("Helvetica", "I", 8)
					pdf.SetTextColor(120, 120, 120)
					currency := "USD"
					if item.CostCurrency.Valid && item.CostCurrency.String != "" {
						currency = item.CostCurrency.String
					}
					costStr := fmt.Sprintf("Est. cost: %s %.2f",
						currency, float64(item.EstimatedCostCents.Int64)/100)
					pdf.Cell(0, 4, costStr)
					pdf.Ln(4)
					pdf.SetTextColor(0, 0, 0)
				}

				pdf.Ln(3)
			}
			pdf.Ln(2)
		}
	}

	// --- Bookings ---
	if len(bookings) > 0 {
		pdf.SetDrawColor(200, 200, 200)
		pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
		pdf.Ln(6)

		pdf.SetFont("Helvetica", "B", 14)
		pdf.Cell(0, 8, "Bookings")
		pdf.Ln(10)

		for _, b := range bookings {
			pdf.SetFont("Helvetica", "B", 10)
			typeLabel := b.Type
			if typeLabel == "" {
				typeLabel = "Booking"
			}
			pdf.Cell(0, 5, safeString(fmt.Sprintf("[%s] %s", typeLabel, b.Title)))
			pdf.Ln(5)

			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(80, 80, 80)

			if b.Provider.Valid && b.Provider.String != "" {
				pdf.Cell(0, 4, "Provider: "+safeString(b.Provider.String))
				pdf.Ln(4)
			}
			if b.ConfirmationCode.Valid && b.ConfirmationCode.String != "" {
				pdf.Cell(0, 4, "Confirmation: "+safeString(b.ConfirmationCode.String))
				pdf.Ln(4)
			}
			if b.StartTime.Valid {
				timeStr := "Date: " + b.StartTime.Time.Format("Jan 2, 2006 3:04 PM")
				if b.EndTime.Valid {
					timeStr += " - " + b.EndTime.Time.Format("Jan 2, 2006 3:04 PM")
				}
				pdf.Cell(0, 4, timeStr)
				pdf.Ln(4)
			}

			pdf.SetTextColor(0, 0, 0)
			pdf.Ln(4)
		}
	}

	// --- Footer ---
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(150, 150, 150)
	pdf.Ln(10)
	pdf.Cell(0, 4, fmt.Sprintf("Generated by Toqui - toqui.travel | %s",
		time.Now().UTC().Format("Jan 2, 2006 3:04 PM UTC")))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// safeString replaces non-latin1 characters that fpdf can't handle.
func safeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 256 {
			b.WriteRune(r)
		} else {
			b.WriteRune('?')
		}
	}
	return b.String()
}
