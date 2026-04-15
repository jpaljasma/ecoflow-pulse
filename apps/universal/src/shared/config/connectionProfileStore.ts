import AsyncStorage from '@react-native-async-storage/async-storage';

import {
  defaultConnectionProfileId,
  env,
  type ConnectionProfileId,
  isConnectionProfileConfigured,
  setActiveConnectionProfileId
} from '@/shared/config/env';
import { create, createJSONStorage, persist } from '@/shared/state/zustand';

type ConnectionProfileState = {
  hydrated: boolean;
  profileId: ConnectionProfileId;
  setHydrated: (hydrated: boolean) => void;
  setProfileId: (profileId: ConnectionProfileId) => void;
};

function normalizeProfileId(value: unknown): ConnectionProfileId {
  if (value === 'cloud' && isConnectionProfileConfigured('cloud')) {
    return 'cloud';
  }
  if (value === 'local' && isConnectionProfileConfigured('local')) {
    return 'local';
  }
  return defaultConnectionProfileId;
}

function applyProfileSelection(profileId: ConnectionProfileId): ConnectionProfileId {
  const normalized = normalizeProfileId(profileId);
  env.connectionProfileId = normalized;
  setActiveConnectionProfileId(normalized);
  return normalized;
}

export const useConnectionProfileStore = create<ConnectionProfileState>()(
  persist(
    (set) => ({
      hydrated: false,
      profileId: applyProfileSelection(defaultConnectionProfileId),
      setHydrated: (hydrated) => set({ hydrated }),
      setProfileId: (profileId) =>
        set(() => ({
          profileId: applyProfileSelection(profileId)
        }))
    }),
    {
      name: 'pulse-connection-profile-v1',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({ profileId: state.profileId }),
      migrate: (persistedState) => {
        const state = (persistedState ?? {}) as Partial<{ profileId: ConnectionProfileId }>;
        return {
          profileId: normalizeProfileId(state.profileId)
        };
      },
      onRehydrateStorage: () => (state) => {
        if (!state) {
          return;
        }
        state.setProfileId(state.profileId);
        state.setHydrated(true);
      }
    }
  )
);
