import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';
import { isValidIanaTimezone } from '@/features/profile/timezone';

const WeatherLocationSchema = z.object({
  label: z.string().optional(),
  latitude: z.number(),
  longitude: z.number()
});

export const CurrentUserSchema = z.object({
  id: z.string(),
  email: z.string(),
  emailVerified: z.boolean(),
  displayName: z.string(),
  avatarUrl: z.string(),
  authMethod: z.string(),
  givenName: z.string(),
  familyName: z.string(),
  locale: z.string(),
  timezone: z.string(),
  weatherLocationEnabled: z.boolean(),
  weatherLocation: WeatherLocationSchema.nullable()
});

export const CurrentUserBootstrapSchema = z.object({
  user: CurrentUserSchema,
  authorization: z.object({
    roles: z.array(z.string()),
    deviceCount: z.number().int().nonnegative()
  })
});

export const UpdateCurrentUserPayloadSchema = z.object({
  displayName: z.string().trim().min(1).max(120),
  timezone: z
    .string()
    .trim()
    .min(1)
    .max(128)
    .refine((value) => isValidIanaTimezone(value), 'timezone must be a valid IANA timezone'),
  weatherLocationEnabled: z.boolean(),
  weatherLocation: WeatherLocationSchema.nullable().optional()
});

export type CurrentUser = z.infer<typeof CurrentUserSchema>;
export type CurrentUserBootstrap = z.infer<typeof CurrentUserBootstrapSchema>;
export type UpdateCurrentUserPayload = z.infer<typeof UpdateCurrentUserPayloadSchema>;

export async function fetchCurrentUser(token?: string): Promise<CurrentUserBootstrap> {
  const data = await requestJson<unknown>('/api/v1/me', { token });
  return CurrentUserBootstrapSchema.parse(data);
}

export async function updateCurrentUser(
  payload: UpdateCurrentUserPayload,
  token?: string
): Promise<CurrentUser> {
  const validated = UpdateCurrentUserPayloadSchema.parse(payload);
  const data = await requestJson<unknown>('/api/v1/me', {
    method: 'PATCH',
    token,
    body: validated
  });
  return z.object({ user: CurrentUserSchema }).parse(data).user;
}

export async function refreshCurrentUserIdentity(token?: string): Promise<CurrentUser> {
  const data = await requestJson<unknown>('/api/v1/me/identity-refresh', {
    method: 'POST',
    token
  });
  return z.object({ user: CurrentUserSchema }).parse(data).user;
}
