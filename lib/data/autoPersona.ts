import type { PersonaIntroData } from "@/lib/hooks/useChat";

/**
 * Maps common destination keywords to a sensible default persona so we can
 * show an intro card on the very first chat screen — before the backend has
 * had a chance to assign one via the AI response.
 *
 * The personas here are a small, hand-picked subset that cover the most
 * popular trip templates. Once the backend assigns a real persona, the
 * PersonaIntroCard rendered from the streaming response will take over.
 */
const DESTINATION_PERSONAS: {
  keywords: string[];
  persona: PersonaIntroData;
}[] = [
  {
    keywords: ["tokyo", "japan", "osaka", "kyoto"],
    persona: {
      name: "Tokyo Food Expert",
      specialties: ["Street food", "Ramen", "Izakaya culture"],
      accentColor: "#e74c3c",
      avatarUrl: "",
      handoffMessage:
        "I know every back-alley ramen shop and Michelin-starred sushi counter in Tokyo. Let's plan your food adventure!",
    },
  },
  {
    keywords: ["paris", "france", "lyon", "nice"],
    persona: {
      name: "Paris Architecture Guide",
      specialties: ["Art Nouveau", "Gothic cathedrals", "Hidden courtyards"],
      accentColor: "#2563eb",
      avatarUrl: "",
      handoffMessage:
        "From Haussmann boulevards to secret passages, I'll show you Paris through its stunning architecture.",
    },
  },
  {
    keywords: ["bali", "indonesia", "ubud", "seminyak"],
    persona: {
      name: "Bali Adventure Specialist",
      specialties: ["Surfing", "Temple treks", "Rice terrace hikes"],
      accentColor: "#16a34a",
      avatarUrl: "",
      handoffMessage:
        "Whether you want sunrise volcano hikes or hidden waterfalls, I'll help you discover the real Bali.",
    },
  },
  {
    keywords: ["rome", "italy", "florence", "venice", "milan", "italian"],
    persona: {
      name: "Rome History Guide",
      specialties: ["Ancient ruins", "Vatican tours", "Roman cuisine"],
      accentColor: "#d97706",
      avatarUrl: "",
      handoffMessage:
        "Two thousand years of history and the world's best pasta — I'll help you experience both.",
    },
  },
  {
    keywords: ["new york", "nyc", "manhattan", "brooklyn"],
    persona: {
      name: "NYC Local Expert",
      specialties: ["Neighborhoods", "Broadway", "Hidden speakeasies"],
      accentColor: "#7c3aed",
      avatarUrl: "",
      handoffMessage:
        "Skip the tourist traps — I'll show you the New York that locals love.",
    },
  },
  {
    keywords: ["london", "england", "uk", "british"],
    persona: {
      name: "London Culture Guide",
      specialties: ["Museums", "Theatre", "Pub crawls"],
      accentColor: "#0891b2",
      avatarUrl: "",
      handoffMessage:
        "From the West End to Borough Market, I know every corner of this city. Let's plan your London trip!",
    },
  },
  {
    keywords: ["barcelona", "spain", "madrid"],
    persona: {
      name: "Barcelona Arts Guide",
      specialties: ["Gaudi landmarks", "Tapas bars", "Beach culture"],
      accentColor: "#dc2626",
      avatarUrl: "",
      handoffMessage:
        "Gothic Quarter alleys, Sagrada Familia, and the best patatas bravas in town — I've got you covered.",
    },
  },
  {
    keywords: ["thailand", "bangkok", "chiang mai", "phuket"],
    persona: {
      name: "Thailand Travel Expert",
      specialties: ["Temple trails", "Street food", "Island hopping"],
      accentColor: "#ea580c",
      avatarUrl: "",
      handoffMessage:
        "From bustling Bangkok markets to serene northern temples, let's plan your perfect Thailand adventure.",
    },
  },
  {
    keywords: ["greece", "santorini", "athens", "mykonos", "greek"],
    persona: {
      name: "Greek Islands Guide",
      specialties: ["Island hopping", "Ancient history", "Seafood"],
      accentColor: "#1d4ed8",
      avatarUrl: "",
      handoffMessage:
        "Crystal-clear waters, whitewashed villages, and legendary sunsets — I'll plan your perfect island-hop.",
    },
  },
  {
    keywords: ["morocco", "marrakech", "fes"],
    persona: {
      name: "Morocco Discovery Guide",
      specialties: ["Medina tours", "Sahara treks", "Riad stays"],
      accentColor: "#b45309",
      avatarUrl: "",
      handoffMessage:
        "From the souks of Marrakech to the dunes of the Sahara, I'll help you navigate this incredible country.",
    },
  },
];

/** Default fallback persona when the destination doesn't match any known keywords. */
const DEFAULT_PERSONA: PersonaIntroData = {
  name: "Travel Planning Expert",
  specialties: ["Itinerary design", "Local tips", "Hidden gems"],
  accentColor: "#6366f1",
  avatarUrl: "",
  handoffMessage:
    "I'm your dedicated travel expert — tell me about your trip and I'll help you plan an unforgettable experience.",
};

/**
 * Given a trip title (which often contains the destination), return a
 * relevant auto-assigned persona for the first-chat intro card.
 */
export function getAutoPersona(tripTitle: string | undefined): PersonaIntroData {
  if (!tripTitle) return DEFAULT_PERSONA;
  const lower = tripTitle.toLowerCase();
  for (const entry of DESTINATION_PERSONAS) {
    if (entry.keywords.some((kw) => lower.includes(kw))) {
      return entry.persona;
    }
  }
  return DEFAULT_PERSONA;
}
