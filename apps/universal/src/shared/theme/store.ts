import AsyncStorage from '@react-native-async-storage/async-storage';

import { defaultThemeFamily, type ThemeFamily, type ThemeVariant } from './catalog';
import { create, createJSONStorage, persist } from '@/shared/state/zustand';

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
