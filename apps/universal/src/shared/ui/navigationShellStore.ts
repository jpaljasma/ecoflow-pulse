import AsyncStorage from '@react-native-async-storage/async-storage';
import { create, createJSONStorage, persist } from '@/shared/state/zustand';

type NavigationShellState = {
  hydrated: boolean;
  sidebarExpanded: boolean;
  setHydrated: (hydrated: boolean) => void;
  setSidebarExpanded: (expanded: boolean) => void;
  toggleSidebarExpanded: () => void;
};

export const useNavigationShellStore = create<NavigationShellState>()(
  persist(
    (set) => ({
      hydrated: false,
      sidebarExpanded: true,
      setHydrated: (hydrated) => set({ hydrated }),
      setSidebarExpanded: (sidebarExpanded) => set({ sidebarExpanded }),
      toggleSidebarExpanded: () =>
        set((state) => ({
          sidebarExpanded: !state.sidebarExpanded
        }))
    }),
    {
      name: 'pulse-navigation-shell-v1',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({ sidebarExpanded: state.sidebarExpanded }),
      onRehydrateStorage: () => (state) => {
        state?.setHydrated(true);
      }
    }
  )
);
