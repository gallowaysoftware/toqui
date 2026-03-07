import type { Trip, Itinerary } from "@/gen/toqui/v1/trip_pb";

/** Map itinerary item types to emoji-style text labels for print */
const typeLabels: Record<string, string> = {
  activity: "Activity",
  food: "Food",
  restaurant: "Restaurant",
  dining: "Dining",
  sightseeing: "Sightseeing",
  attraction: "Attraction",
  museum: "Museum",
  shopping: "Shopping",
  accommodation: "Accommodation",
  hotel: "Hotel",
  transport: "Transport",
  flight: "Flight",
  transit: "Transit",
};

function getTypeLabel(type?: string): string {
  if (!type) return "";
  return typeLabels[type.toLowerCase()] || type.charAt(0).toUpperCase() + type.slice(1);
}

/**
 * Build a print-optimized HTML string for the trip itinerary.
 * This opens a new window with the content and triggers window.print().
 */
function buildPrintHTML(trip: Trip, itinerary: Itinerary): string {
  const dateRange =
    trip.startDate && trip.endDate
      ? `${trip.startDate} to ${trip.endDate}`
      : trip.startDate || "";

  const daysHTML = itinerary.days
    .map((day) => {
      const itemsHTML = day.items
        .map((item) => {
          const typeTag = item.type
            ? `<span class="item-type">${getTypeLabel(item.type)}</span>`
            : "";
          const desc = item.description
            ? `<p class="item-desc">${escapeHTML(item.description)}</p>`
            : "";
          return `
            <div class="item">
              <div class="item-header">
                <span class="item-title">${escapeHTML(item.title)}</span>
                ${typeTag}
              </div>
              ${desc}
            </div>
          `;
        })
        .join("");

      return `
        <div class="day">
          <h2>Day ${day.dayNumber}${day.date ? ` - ${day.date}` : ""}</h2>
          ${day.summary ? `<p class="day-summary">${escapeHTML(day.summary)}</p>` : ""}
          <div class="items">${itemsHTML}</div>
        </div>
      `;
    })
    .join("");

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>${escapeHTML(trip.title)} - Toqui Itinerary</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      color: #1a1a1a;
      line-height: 1.6;
      padding: 40px;
      max-width: 800px;
      margin: 0 auto;
    }
    .header {
      border-bottom: 2px solid #E8654A;
      padding-bottom: 20px;
      margin-bottom: 30px;
    }
    .header .brand {
      font-size: 14px;
      color: #E8654A;
      font-weight: 700;
      letter-spacing: 0.5px;
      margin-bottom: 8px;
    }
    .header h1 {
      font-size: 28px;
      font-weight: 700;
      color: #111;
      margin-bottom: 6px;
    }
    .header .description {
      font-size: 14px;
      color: #555;
      margin-bottom: 8px;
    }
    .header .dates {
      font-size: 13px;
      color: #777;
    }
    .day {
      margin-bottom: 28px;
      page-break-inside: avoid;
    }
    .day h2 {
      font-size: 18px;
      font-weight: 600;
      color: #E8654A;
      margin-bottom: 8px;
      padding-bottom: 6px;
      border-bottom: 1px solid #eee;
    }
    .day-summary {
      font-size: 13px;
      color: #666;
      margin-bottom: 10px;
      font-style: italic;
    }
    .items {
      padding-left: 12px;
    }
    .item {
      margin-bottom: 12px;
      padding: 8px 12px;
      border-left: 3px solid #E8654A;
      background: #fafafa;
    }
    .item-header {
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .item-title {
      font-weight: 600;
      font-size: 14px;
    }
    .item-type {
      font-size: 11px;
      color: #888;
      background: #eee;
      padding: 1px 8px;
      border-radius: 10px;
    }
    .item-desc {
      font-size: 13px;
      color: #555;
      margin-top: 4px;
    }
    @media print {
      body { padding: 20px; }
      .day { page-break-inside: avoid; }
    }
  </style>
</head>
<body>
  <div class="header">
    <div class="brand">TOQUI</div>
    <h1>${escapeHTML(trip.title)}</h1>
    ${trip.description ? `<p class="description">${escapeHTML(trip.description)}</p>` : ""}
    ${dateRange ? `<p class="dates">${escapeHTML(dateRange)}</p>` : ""}
  </div>
  ${daysHTML}
</body>
</html>`;
}

function escapeHTML(str: string): string {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/**
 * Export itinerary as a print-optimized PDF via the browser's print dialog.
 * Opens a new window with the formatted itinerary and triggers print.
 */
export function exportItineraryPDF(trip: Trip, itinerary: Itinerary): void {
  const html = buildPrintHTML(trip, itinerary);
  const printWindow = window.open("", "_blank");
  if (!printWindow) {
    // Popup blocked — fall back to printing current window
    const iframe = document.createElement("iframe");
    iframe.style.position = "fixed";
    iframe.style.right = "0";
    iframe.style.bottom = "0";
    iframe.style.width = "0";
    iframe.style.height = "0";
    iframe.style.border = "none";
    document.body.appendChild(iframe);

    const doc = iframe.contentDocument;
    if (doc) {
      doc.open();
      doc.write(html);
      doc.close();
      iframe.contentWindow?.focus();
      iframe.contentWindow?.print();
    }

    // Clean up after print dialog closes
    setTimeout(() => {
      document.body.removeChild(iframe);
    }, 1000);
    return;
  }

  printWindow.document.open();
  printWindow.document.write(html);
  printWindow.document.close();

  // Wait for content to render before triggering print
  printWindow.onload = () => {
    printWindow.focus();
    printWindow.print();
  };

  // Fallback: if onload doesn't fire (some browsers)
  setTimeout(() => {
    printWindow.focus();
    printWindow.print();
  }, 500);
}

// For testing
export { buildPrintHTML, getTypeLabel };
