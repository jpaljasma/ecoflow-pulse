import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  fetchCurrentUser,
  refreshCurrentUserIdentity,
  updateCurrentUser,
  type CurrentUser,
  type CurrentUserBootstrap,
  type UpdateCurrentUserPayload
} from '@/features/profile/api';
import { didWeatherProfileInputsChange, mergeCurrentUserBootstrap } from '@/features/profile/model';

type ProfileQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
};

export function useCurrentUser(options: ProfileQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery<CurrentUserBootstrap>({
    queryKey: ['current-user', authKey],
    queryFn: () => fetchCurrentUser(token),
    enabled,
    staleTime: 60_000,
    gcTime: 5 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useUpdateCurrentUser(options: ProfileQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateCurrentUserPayload) => updateCurrentUser(payload, token),
    onSuccess: (user: CurrentUser) => {
      const queryKey = ['current-user', authKey] as const;
      const previous = queryClient.getQueryData<CurrentUserBootstrap>(queryKey);
      queryClient.setQueryData<CurrentUserBootstrap>(queryKey, (cached) => {
        return mergeCurrentUserBootstrap(cached, user);
      });
      if (didWeatherProfileInputsChange(previous?.user, user)) {
        void queryClient.invalidateQueries({
          queryKey: ['weather', authKey]
        });
      }
    }
  });
}

export function useRefreshCurrentUserIdentity(options: ProfileQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => refreshCurrentUserIdentity(token),
    onSuccess: (user: CurrentUser) => {
      queryClient.setQueryData<CurrentUserBootstrap>(['current-user', authKey], (previous) => {
        return mergeCurrentUserBootstrap(previous, user);
      });
    }
  });
}
