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
  defaultProfileId: ConnectionProfileId;
  setHydrated: (hydrated: boolean) => void;
  setProfileId: (profileId: ConnectionProfileId) => void;
};

type PersistedConnectionProfileState = Partial<{
  profileId: ConnectionProfileId;
  defaultProfileId: ConnectionProfileId;
}>;

function readPersistedProfileId(value: unknown): ConnectionProfileId | null {
  if (value === 'cloud' || value === 'local') {
    return value;
  }
  return null;
}

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

function resolvePersistedProfileId(
  value: unknown,
  version: number | undefined,
  persistedDefaultProfileId: unknown
): ConnectionProfileId {
  const persistedDefault = readPersistedProfileId(persistedDefaultProfileId);
  if (persistedDefault && persistedDefault !== defaultConnectionProfileId) {
    return defaultConnectionProfileId;
  }

  if (
    !persistedDefault &&
    value !== undefined &&
    normalizeProfileId(value) !== defaultConnectionProfileId &&
    (version === undefined || version >= 3)
  ) {
    return defaultConnectionProfileId;
  }

  if (
    version !== undefined &&
    version < 2 &&
    value === 'local' &&
    defaultConnectionProfileId === 'cloud'
  ) {
    return 'cloud';
  }
  if (
    version !== undefined &&
    version < 3 &&
    value === 'cloud' &&
    defaultConnectionProfileId === 'local'
  ) {
    return 'local';
  }
  return normalizeProfileId(value);
}

export const useConnectionProfileStore = create<ConnectionProfileState>()(
  persist(
    (set) => ({
      hydrated: false,
      profileId: applyProfileSelection(defaultConnectionProfileId),
      defaultProfileId: defaultConnectionProfileId,
      setHydrated: (hydrated) => set({ hydrated }),
      setProfileId: (profileId) =>
        set(() => ({
          profileId: applyProfileSelection(profileId),
          defaultProfileId: defaultConnectionProfileId
        }))
    }),
    {
      name: 'pulse-connection-profile-v1',
      version: 3,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({
        profileId: state.profileId,
        defaultProfileId: state.defaultProfileId
      }),
      migrate: (persistedState, version) => {
        const state = (persistedState ?? {}) as PersistedConnectionProfileState;
        return {
          profileId: resolvePersistedProfileId(state.profileId, version, state.defaultProfileId),
          defaultProfileId: defaultConnectionProfileId
        };
      },
      onRehydrateStorage: () => (state) => {
        if (!state) {
          return;
        }
        state.setProfileId(
          resolvePersistedProfileId(state.profileId, undefined, state.defaultProfileId)
        );
        state.setHydrated(true);
      }
    }
  )
);
