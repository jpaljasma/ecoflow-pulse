import AsyncStorage from '@react-native-async-storage/async-storage';

import { create, createJSONStorage, persist } from '@/shared/state/zustand';

type EnergySettingsState = {
  hydrated: boolean;
  gridPricePerKwh: string;
  currency: string;
  setHydrated: (hydrated: boolean) => void;
  setGridPricePerKwh: (value: string) => void;
  setCurrency: (value: string) => void;
};

export const useEnergySettingsStore = create<EnergySettingsState>()(
  persist(
    (set) => ({
      hydrated: false,
      gridPricePerKwh: '0.30',
      currency: 'USD',
      setHydrated: (hydrated) => set({ hydrated }),
      setGridPricePerKwh: (gridPricePerKwh) => set({ gridPricePerKwh }),
      setCurrency: (currency) => set({ currency })
    }),
    {
      name: 'pulse-energy-settings-v1',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({
        gridPricePerKwh: state.gridPricePerKwh,
        currency: state.currency
      }),
      onRehydrateStorage: () => (state) => {
        state?.setHydrated(true);
      }
    }
  )
);
