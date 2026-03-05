import { create } from "zustand";

interface LocationState {
  latitude: number | null;
  longitude: number | null;
  accuracy: number | null;
  lastUpdated: Date | null;
  isTracking: boolean;
  setLocation: (lat: number, lng: number, accuracy: number) => void;
  setTracking: (tracking: boolean) => void;
}

export const useLocationStore = create<LocationState>((set) => ({
  latitude: null,
  longitude: null,
  accuracy: null,
  lastUpdated: null,
  isTracking: false,
  setLocation: (latitude, longitude, accuracy) =>
    set({ latitude, longitude, accuracy, lastUpdated: new Date() }),
  setTracking: (isTracking) => set({ isTracking }),
}));
