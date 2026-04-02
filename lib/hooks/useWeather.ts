import { useQuery } from "@tanstack/react-query";

export interface WeatherDay {
  date: string;
  tempHigh: number;
  tempLow: number;
  precipitation: number;
  weatherCode: number;
  description: string;
}

/** WMO Weather interpretation codes → human description */
const WMO_DESCRIPTIONS: Record<number, string> = {
  0: "Clear sky",
  1: "Mainly clear",
  2: "Partly cloudy",
  3: "Overcast",
  45: "Fog",
  48: "Depositing rime fog",
  51: "Light drizzle",
  53: "Moderate drizzle",
  55: "Dense drizzle",
  56: "Light freezing drizzle",
  57: "Dense freezing drizzle",
  61: "Slight rain",
  63: "Moderate rain",
  65: "Heavy rain",
  66: "Light freezing rain",
  67: "Heavy freezing rain",
  71: "Slight snow",
  73: "Moderate snow",
  75: "Heavy snow",
  77: "Snow grains",
  80: "Slight rain showers",
  81: "Moderate rain showers",
  82: "Violent rain showers",
  85: "Slight snow showers",
  86: "Heavy snow showers",
  95: "Thunderstorm",
  96: "Thunderstorm with slight hail",
  99: "Thunderstorm with heavy hail",
};

export function getWeatherDescription(code: number): string {
  return WMO_DESCRIPTIONS[code] ?? "Unknown";
}

export function getWeatherEmoji(code: number): string {
  if (code === 0 || code === 1) return "\u2600\uFE0F"; // sun
  if (code === 2) return "\u26C5";                      // sun behind cloud
  if (code === 3) return "\u2601\uFE0F";                // cloud
  if (code === 45 || code === 48) return "\u{1F32B}\uFE0F"; // fog
  if (code >= 51 && code <= 57) return "\u{1F327}\uFE0F";   // rain
  if (code >= 61 && code <= 67) return "\u{1F327}\uFE0F";   // rain
  if (code >= 71 && code <= 77) return "\u2744\uFE0F";       // snowflake
  if (code >= 80 && code <= 82) return "\u{1F326}\uFE0F";   // rain showers
  if (code >= 85 && code <= 86) return "\u{1F328}\uFE0F";   // snow showers
  if (code >= 95) return "\u26C8\uFE0F";                     // thunderstorm
  return "\u2601\uFE0F";
}

export function celsiusToFahrenheit(c: number): number {
  return Math.round((c * 9) / 5 + 32);
}

interface ForecastApiResponse {
  daily: {
    time: string[];
    temperature_2m_max: number[];
    temperature_2m_min: number[];
    precipitation_sum: number[];
    weathercode?: number[];
  };
}

interface ClimateApiResponse {
  daily: {
    time: string[];
    temperature_2m_max: number[];
    temperature_2m_min: number[];
    precipitation_sum: number[];
  };
}

/**
 * Determines if we should use the climate API (historical averages)
 * instead of the forecast API. The forecast API only covers ~16 days out.
 */
function shouldUseClimateApi(startDate: string): boolean {
  const start = new Date(`${startDate}T00:00:00Z`);
  const now = new Date();
  const daysOut = Math.ceil((start.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
  return daysOut > 16;
}

export function buildWeatherUrl(
  latitude: number,
  longitude: number,
  startDate: string,
  endDate: string,
): string {
  if (shouldUseClimateApi(startDate)) {
    return `https://climate-api.open-meteo.com/v1/climate?latitude=${latitude}&longitude=${longitude}&start_date=${startDate}&end_date=${endDate}&daily=temperature_2m_max,temperature_2m_min,precipitation_sum`;
  }
  return `https://api.open-meteo.com/v1/forecast?latitude=${latitude}&longitude=${longitude}&daily=temperature_2m_max,temperature_2m_min,precipitation_sum,weathercode&start_date=${startDate}&end_date=${endDate}&timezone=auto`;
}

function parseWeatherResponse(
  data: ForecastApiResponse | ClimateApiResponse,
  isClimate: boolean,
): WeatherDay[] {
  const { time, temperature_2m_max, temperature_2m_min, precipitation_sum } = data.daily;
  const weathercodes = isClimate ? undefined : (data as ForecastApiResponse).daily.weathercode;

  return time.map((date, i) => ({
    date,
    tempHigh: Math.round(temperature_2m_max[i]),
    tempLow: Math.round(temperature_2m_min[i]),
    precipitation: precipitation_sum[i],
    weatherCode: weathercodes?.[i] ?? -1,
    description:
      weathercodes?.[i] != null
        ? getWeatherDescription(weathercodes[i])
        : precipitation_sum[i] > 1
          ? "Possible rain"
          : "Typical",
  }));
}

export function useWeather(
  latitude: number | null,
  longitude: number | null,
  startDate: string | null,
  endDate: string | null,
) {
  const enabled =
    latitude != null &&
    longitude != null &&
    startDate != null &&
    endDate != null;

  const isClimate = enabled ? shouldUseClimateApi(startDate!) : false;

  const { data: weather, isLoading, error } = useQuery<WeatherDay[]>({
    queryKey: ["weather", latitude, longitude, startDate, endDate, isClimate],
    queryFn: async () => {
      const url = buildWeatherUrl(latitude!, longitude!, startDate!, endDate!);
      const res = await fetch(url);
      if (!res.ok) throw new Error(`Weather API error: ${res.status}`);
      const json = await res.json();
      return parseWeatherResponse(json, isClimate);
    },
    enabled,
    staleTime: 60 * 60 * 1000, // 1 hour
    retry: 1,
  });

  return { weather: weather ?? null, isLoading, isClimate, error };
}
