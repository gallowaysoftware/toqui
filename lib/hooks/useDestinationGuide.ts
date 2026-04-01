import { useQuery } from "@tanstack/react-query";
import { getConfig } from "@/lib/config";

export interface DestinationGuide {
  slug: string;
  title: string;
  persona_name: string;
  persona_specialty: string;
  destination: string;
  country: string;
  theme: string;
  excerpt: string;
  content: string;
  cta_text: string;
  cta_url: string;
}

interface GuidesListResponse {
  guides: DestinationGuide[];
  total: number;
}

export function useDestinationGuide(destinationCountry: string | undefined): {
  guide: DestinationGuide | null;
  isLoading: boolean;
} {
  const enabled = !!destinationCountry;

  const { data, isLoading } = useQuery({
    queryKey: ["destinationGuides"],
    queryFn: async (): Promise<GuidesListResponse> => {
      const res = await fetch(`${getConfig().apiUrl}/api/guides`);
      if (!res.ok) return { guides: [], total: 0 };
      return res.json();
    },
    enabled,
    staleTime: 60 * 60 * 1000, // 1 hour — guides don't change often
    retry: false,
  });

  if (!enabled || isLoading || !data) {
    return { guide: null, isLoading: enabled && isLoading };
  }

  const countryUpper = destinationCountry.toUpperCase();
  const match =
    data.guides.find((g) => g.country.toUpperCase() === countryUpper) ?? null;

  return { guide: match, isLoading: false };
}
