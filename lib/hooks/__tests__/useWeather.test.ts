import { describe, it, expect } from "vitest";
import {
  buildWeatherUrl,
  getWeatherDescription,
  getWeatherEmoji,
  celsiusToFahrenheit,
} from "../useWeather";

describe("buildWeatherUrl", () => {
  it("uses forecast API for near-future dates", () => {
    // Use tomorrow's date to ensure it's always within 16 days
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    const startDate = tomorrow.toISOString().split("T")[0];
    const endDate = startDate;

    const url = buildWeatherUrl(35.68, 139.69, startDate, endDate);
    expect(url).toContain("api.open-meteo.com/v1/forecast");
    expect(url).toContain("latitude=35.68");
    expect(url).toContain("longitude=139.69");
    expect(url).toContain(`start_date=${startDate}`);
    expect(url).toContain(`end_date=${endDate}`);
    expect(url).toContain("weathercode");
    expect(url).toContain("timezone=auto");
  });

  it("uses climate API for far-future dates", () => {
    // Use a date 30 days from now
    const future = new Date();
    future.setDate(future.getDate() + 30);
    const startDate = future.toISOString().split("T")[0];
    const endDate = startDate;

    const url = buildWeatherUrl(48.85, 2.35, startDate, endDate);
    expect(url).toContain("climate-api.open-meteo.com/v1/climate");
    expect(url).toContain("latitude=48.85");
    expect(url).toContain("longitude=2.35");
    expect(url).not.toContain("weathercode");
  });

  it("includes all required daily parameters for forecast", () => {
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    const startDate = tomorrow.toISOString().split("T")[0];

    const url = buildWeatherUrl(0, 0, startDate, startDate);
    expect(url).toContain("temperature_2m_max");
    expect(url).toContain("temperature_2m_min");
    expect(url).toContain("precipitation_sum");
  });
});

describe("getWeatherDescription", () => {
  it("returns correct descriptions for known codes", () => {
    expect(getWeatherDescription(0)).toBe("Clear sky");
    expect(getWeatherDescription(2)).toBe("Partly cloudy");
    expect(getWeatherDescription(61)).toBe("Slight rain");
    expect(getWeatherDescription(95)).toBe("Thunderstorm");
  });

  it("returns Unknown for unrecognized codes", () => {
    expect(getWeatherDescription(999)).toBe("Unknown");
  });
});

describe("getWeatherEmoji", () => {
  it("returns sun emoji for clear sky", () => {
    expect(getWeatherEmoji(0)).toBe("\u2600\uFE0F");
    expect(getWeatherEmoji(1)).toBe("\u2600\uFE0F");
  });

  it("returns cloud emoji for overcast", () => {
    expect(getWeatherEmoji(3)).toBe("\u2601\uFE0F");
  });

  it("returns rain emoji for rain codes", () => {
    expect(getWeatherEmoji(61)).toBe("\u{1F327}\uFE0F");
    expect(getWeatherEmoji(63)).toBe("\u{1F327}\uFE0F");
  });

  it("returns snowflake for snow codes", () => {
    expect(getWeatherEmoji(71)).toBe("\u2744\uFE0F");
    expect(getWeatherEmoji(75)).toBe("\u2744\uFE0F");
  });

  it("returns thunderstorm emoji for storm codes", () => {
    expect(getWeatherEmoji(95)).toBe("\u26C8\uFE0F");
    expect(getWeatherEmoji(99)).toBe("\u26C8\uFE0F");
  });
});

describe("celsiusToFahrenheit", () => {
  it("converts 0°C to 32°F", () => {
    expect(celsiusToFahrenheit(0)).toBe(32);
  });

  it("converts 100°C to 212°F", () => {
    expect(celsiusToFahrenheit(100)).toBe(212);
  });

  it("converts 25°C to 77°F", () => {
    expect(celsiusToFahrenheit(25)).toBe(77);
  });

  it("rounds the result", () => {
    // 22°C = 71.6°F → 72
    expect(celsiusToFahrenheit(22)).toBe(72);
  });
});
