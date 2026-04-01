export type TemplateCategory = "weekend" | "week" | "adventure" | "family" | "romantic" | "cultural";

export interface TripTemplate {
  id: string;
  titleKey: string;
  destination: string;
  destinationCountry: string;
  duration: number;
  descriptionKey: string;
  category: TemplateCategory;
  icon: string;
  accentColor: string;
  suggestedPromptKey: string;
}

export const TEMPLATE_CATEGORIES: TemplateCategory[] = [
  "weekend",
  "week",
  "adventure",
  "family",
  "romantic",
  "cultural",
];

export const tripTemplates: TripTemplate[] = [
  {
    id: "paris-weekend",
    titleKey: "templates.items.parisWeekend.title",
    destination: "Paris",
    destinationCountry: "France",
    duration: 3,
    descriptionKey: "templates.items.parisWeekend.description",
    category: "romantic",
    icon: "Heart",
    accentColor: "#E91E63",
    suggestedPromptKey: "templates.items.parisWeekend.suggestedPrompt",
  },
  {
    id: "tokyo-explorer",
    titleKey: "templates.items.tokyoExplorer.title",
    destination: "Tokyo",
    destinationCountry: "Japan",
    duration: 7,
    descriptionKey: "templates.items.tokyoExplorer.description",
    category: "cultural",
    icon: "Landmark",
    accentColor: "#9C27B0",
    suggestedPromptKey: "templates.items.tokyoExplorer.suggestedPrompt",
  },
  {
    id: "bali-beach",
    titleKey: "templates.items.baliBeach.title",
    destination: "Bali",
    destinationCountry: "Indonesia",
    duration: 10,
    descriptionKey: "templates.items.baliBeach.description",
    category: "adventure",
    icon: "TreePalm",
    accentColor: "#009688",
    suggestedPromptKey: "templates.items.baliBeach.suggestedPrompt",
  },
  {
    id: "nyc-city",
    titleKey: "templates.items.nycCity.title",
    destination: "New York City",
    destinationCountry: "United States",
    duration: 4,
    descriptionKey: "templates.items.nycCity.description",
    category: "cultural",
    icon: "Building2",
    accentColor: "#3F51B5",
    suggestedPromptKey: "templates.items.nycCity.suggestedPrompt",
  },
  {
    id: "italian-road-trip",
    titleKey: "templates.items.italianRoadTrip.title",
    destination: "Italy",
    destinationCountry: "Italy",
    duration: 14,
    descriptionKey: "templates.items.italianRoadTrip.description",
    category: "adventure",
    icon: "Car",
    accentColor: "#FF5722",
    suggestedPromptKey: "templates.items.italianRoadTrip.suggestedPrompt",
  },
  {
    id: "barcelona-costa-brava",
    titleKey: "templates.items.barcelonaCostaBrava.title",
    destination: "Barcelona",
    destinationCountry: "Spain",
    duration: 5,
    descriptionKey: "templates.items.barcelonaCostaBrava.description",
    category: "weekend",
    icon: "Sun",
    accentColor: "#FF9800",
    suggestedPromptKey: "templates.items.barcelonaCostaBrava.suggestedPrompt",
  },
  {
    id: "thailand-backpacking",
    titleKey: "templates.items.thailandBackpacking.title",
    destination: "Thailand",
    destinationCountry: "Thailand",
    duration: 14,
    descriptionKey: "templates.items.thailandBackpacking.description",
    category: "adventure",
    icon: "Backpack",
    accentColor: "#4CAF50",
    suggestedPromptKey: "templates.items.thailandBackpacking.suggestedPrompt",
  },
  {
    id: "london-family",
    titleKey: "templates.items.londonFamily.title",
    destination: "London",
    destinationCountry: "United Kingdom",
    duration: 7,
    descriptionKey: "templates.items.londonFamily.description",
    category: "family",
    icon: "Users",
    accentColor: "#2196F3",
    suggestedPromptKey: "templates.items.londonFamily.suggestedPrompt",
  },
  {
    id: "greek-islands",
    titleKey: "templates.items.greekIslands.title",
    destination: "Greek Islands",
    destinationCountry: "Greece",
    duration: 10,
    descriptionKey: "templates.items.greekIslands.description",
    category: "romantic",
    icon: "Sailboat",
    accentColor: "#00BCD4",
    suggestedPromptKey: "templates.items.greekIslands.suggestedPrompt",
  },
  {
    id: "morocco-discovery",
    titleKey: "templates.items.moroccoDiscovery.title",
    destination: "Morocco",
    destinationCountry: "Morocco",
    duration: 7,
    descriptionKey: "templates.items.moroccoDiscovery.description",
    category: "cultural",
    icon: "Compass",
    accentColor: "#795548",
    suggestedPromptKey: "templates.items.moroccoDiscovery.suggestedPrompt",
  },
];

export function getTemplateById(id: string): TripTemplate | undefined {
  return tripTemplates.find((t) => t.id === id);
}

export function getTemplatesByCategory(category: TemplateCategory): TripTemplate[] {
  return tripTemplates.filter((t) => t.category === category);
}

export function searchTemplates(query: string): TripTemplate[] {
  const lower = query.toLowerCase();
  return tripTemplates.filter(
    (t) =>
      t.destination.toLowerCase().includes(lower) ||
      t.destinationCountry.toLowerCase().includes(lower),
  );
}
