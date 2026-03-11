import AsyncStorage from '@react-native-async-storage/async-storage';
// eslint-disable-next-line @typescript-eslint/no-require-imports
const { create } = require('zustand') as typeof import('zustand');
// eslint-disable-next-line @typescript-eslint/no-require-imports
const { createJSONStorage, persist } = require('zustand/middleware') as typeof import('zustand/middleware');

import { defaultThemeFamily, type ThemeFamily, type ThemeVariant } from './catalog';

type ThemeState = {
  hydrated: boolean;
  family: ThemeFamily;
  setHydrated: (hydrated: boolean) => void;
  setFamily: (family: ThemeFamily) => void;
};

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      hydrated: false,
      family: defaultThemeFamily,
      setHydrated: (hydrated) => set({ hydrated }),
      setFamily: (family) => set({ family })
    }),
    {
      name: 'pulse-theme-v1',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({ family: state.family }),
      migrate: (persistedState: unknown, version) => {
        const state = (persistedState ?? {}) as Partial<{ family: ThemeFamily; variant: ThemeVariant }>;
        if (state.family === 'original' || state.family === 'new') {
          return { family: state.family };
        }
        if (version < 1 && typeof state.variant === 'string') {
          const family = state.variant.startsWith('original-') ? 'original' : 'new';
          return { family };
        }
        return { family: defaultThemeFamily };
      },
      onRehydrateStorage: () => (state) => {
        state?.setHydrated(true);
      }
    }
  )
);
