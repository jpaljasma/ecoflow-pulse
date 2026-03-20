import type { FastifyInstance, preHandlerHookHandler } from 'fastify';
import { z } from 'zod';

import type { AppConfig } from '../config.js';
import type { ControlPlaneClient } from '../grpc/controlPlaneClient.js';
import { isValidIanaTimezone } from '../timezones.js';
import {
  getAuthHeader,
  getRequestID,
  handleGrpcRouteError,
  loadCurrentUserBootstrap,
  resolveUserSubject
} from './currentUserContext.js';

const updateCurrentUserSchema = z.object({
  displayName: z.string().trim().min(1).max(120),
  timezone: z
    .string()
    .trim()
    .min(1)
    .max(128)
    .refine((value) => isValidIanaTimezone(value), 'timezone must be a valid IANA timezone'),
  weatherLocationEnabled: z.boolean(),
  weatherLocation: z
    .object({
      label: z.string().trim().min(1).max(160).optional(),
      latitude: z.number().finite().gte(-90).lte(90),
      longitude: z.number().finite().gte(-180).lte(180)
    })
    .nullable()
    .optional()
}).superRefine((value, ctx) => {
  if (!value.weatherLocationEnabled && value.weatherLocation !== null && value.weatherLocation !== undefined) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      path: ['weatherLocation'],
      message: 'weatherLocation must be empty when weatherLocationEnabled is false'
    });
  }
});

export function registerCurrentUserRoutes(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  authPreHandler: preHandlerHookHandler
): void {
  app.get('/api/v1/me', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const bootstrap = await loadCurrentUserBootstrap(app, config, controlPlaneClient, request);
      return {
        user: toCurrentUserResponse(bootstrap.user),
        authorization: {
          roles: bootstrap.authorization.tokenRoles,
          deviceCount: bootstrap.authorization.deviceCount
        }
      };
    } catch (error) {
      return handleGrpcRouteError(config, reply, error);
    }
  });

  app.patch('/api/v1/me', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      const body = updateCurrentUserSchema.parse(request.body);
      const weatherLocation = body.weatherLocation ?? undefined;
      const updated = await controlPlaneClient.updateCurrentUser({
        userSubject: resolveUserSubject(config, request),
        displayName: body.displayName,
        timezone: body.timezone,
        weatherLocationEnabled: body.weatherLocationEnabled,
        weatherLocationSource:
          body.weatherLocationEnabled && weatherLocation ? 'auto' : 'none',
        weatherLocationLabel: weatherLocation?.label,
        weatherLatitude: weatherLocation?.latitude,
        weatherLongitude: weatherLocation?.longitude,
        hasWeatherLocation: body.weatherLocationEnabled && Boolean(weatherLocation),
        authHeader: getAuthHeader(request),
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return {
        user: toCurrentUserResponse(updated)
      };
    } catch (error) {
      if (error instanceof z.ZodError) {
        return reply.code(400).send({ error: 'invalid_request', issues: error.issues });
      }
      return handleGrpcRouteError(config, reply, error);
    }
  });

  app.post('/api/v1/me/identity-refresh', { preHandler: authPreHandler }, async (request, reply) => {
    try {
      if (config.auth.mode !== 'keycloak') {
        return reply.code(409).send({
          error: 'identity_refresh_unavailable',
          message: 'Identity refresh requires Keycloak auth mode.'
        });
      }
      const authHeader = getAuthHeader(request);
      if (!authHeader) {
        return reply.code(401).send({ error: 'missing_bearer_token' });
      }
      const profile = await fetchKeycloakUserInfo(resolveUserInfoUrl(config), authHeader);
      const refreshed = await controlPlaneClient.refreshCurrentUserIdentity({
        userSubject: resolveUserSubject(config, request),
        email: profile.email,
        emailVerified: profile.emailVerified,
        displayName: profile.name,
        avatarUrl: profile.picture,
        givenName: profile.givenName,
        familyName: profile.familyName,
        locale: profile.locale,
        authHeader,
        requestID: getRequestID(request),
        deadlineMs: app.telemetryDeadlineMs
      });
      return {
        user: toCurrentUserResponse(refreshed)
      };
    } catch (error) {
      return handleGrpcRouteError(config, reply, error);
    }
  });
}

function toCurrentUserResponse(user: {
  id: string;
  email: string;
  emailVerified: boolean;
  displayName: string;
  avatarUrl: string;
  authMethod: string;
  givenName: string;
  familyName: string;
  locale: string;
  timezone: string;
  weatherLocationEnabled: boolean;
  weatherLocationLabel: string;
  weatherLatitude?: number;
  weatherLongitude?: number;
  hasWeatherLocation: boolean;
}): Record<string, unknown> {
  return {
    id: user.id,
    email: user.email,
    emailVerified: user.emailVerified,
    displayName: user.displayName,
    avatarUrl: user.avatarUrl,
    authMethod: user.authMethod,
    givenName: user.givenName,
    familyName: user.familyName,
    locale: user.locale,
    timezone: user.timezone,
    weatherLocationEnabled: user.weatherLocationEnabled,
    ...(user.hasWeatherLocation
      ? {
          weatherLocation: {
            label: user.weatherLocationLabel,
            latitude: user.weatherLatitude,
            longitude: user.weatherLongitude
          }
        }
      : { weatherLocation: null })
  };
}


const KeycloakUserInfoSchema = z.object({
  email: z.string().trim().optional().default(''),
  email_verified: z.boolean().optional().default(false),
  name: z.string().trim().optional().default(''),
  given_name: z.string().trim().optional().default(''),
  family_name: z.string().trim().optional().default(''),
  locale: z.string().trim().optional().default(''),
  picture: z.string().trim().optional().default('')
});

type KeycloakUserInfo = {
  email: string;
  emailVerified: boolean;
  name: string;
  givenName: string;
  familyName: string;
  locale: string;
  picture: string;
};

function resolveUserInfoUrl(config: AppConfig): string {
  if (config.auth.mode !== 'keycloak') {
    throw new Error('identity_refresh_unavailable');
  }
  if (config.auth.userInfoUrl) {
    return config.auth.userInfoUrl;
  }
  return `${config.auth.issuerUrl.replace(/\/+$/, '')}/protocol/openid-connect/userinfo`;
}

async function fetchKeycloakUserInfo(userInfoUrl: string, authHeader: string): Promise<KeycloakUserInfo> {
  const response = await fetch(userInfoUrl, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
      Authorization: authHeader
    }
  });
  const body = await response.json().catch(() => undefined);
  if (!response.ok) {
    const message =
      typeof body === 'object' && body !== null && 'error_description' in body
        ? String((body as { error_description?: unknown }).error_description ?? 'userinfo request failed')
        : `userinfo request failed (${response.status})`;
    throw new Error(message);
  }
  const parsed = KeycloakUserInfoSchema.parse(body);
  return {
    email: parsed.email,
    emailVerified: parsed.email_verified,
    name: parsed.name,
    givenName: parsed.given_name,
    familyName: parsed.family_name,
    locale: parsed.locale,
    picture: parsed.picture
  };
}
