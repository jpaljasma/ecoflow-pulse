import type { FastifyInstance, FastifyRequest } from 'fastify';
import type { ServiceError } from '@grpc/grpc-js';
import { status as grpcStatus } from '@grpc/grpc-js';

import type { AppConfig } from '../config.js';
import type {
  ControlPlaneClient,
  CurrentUser,
  CurrentUserBootstrap
} from '../grpc/controlPlaneClient.js';

export type WeatherLocation = {
  latitude: number;
  longitude: number;
};

export async function loadCurrentUserBootstrap(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  request: FastifyRequest
): Promise<CurrentUserBootstrap> {
  return await controlPlaneClient.getCurrentUser({
    userSubject: resolveUserSubject(config, request),
    authHeader: getAuthHeader(request),
    requestID: getRequestID(request),
    deadlineMs: app.telemetryDeadlineMs
  });
}

export async function loadWeatherContext(
  app: FastifyInstance,
  config: AppConfig,
  controlPlaneClient: ControlPlaneClient,
  request: FastifyRequest
): Promise<{ location: WeatherLocation; timezone: string } | null> {
  const bootstrap = await loadCurrentUserBootstrap(app, config, controlPlaneClient, request);
  const location = resolveWeatherLocation(bootstrap.user);
  if (!location) {
    return null;
  }
  return {
    location,
    timezone: resolveWeatherTimezone(bootstrap.user.timezone)
  };
}

export function resolveWeatherLocation(user: CurrentUser): WeatherLocation | null {
  if (!user.weatherLocationEnabled || !user.hasWeatherLocation) {
    return null;
  }
  const latitude = user.weatherLatitude;
  const longitude = user.weatherLongitude;
  if (
    typeof latitude !== 'number' ||
    typeof longitude !== 'number' ||
    !Number.isFinite(latitude) ||
    !Number.isFinite(longitude)
  ) {
    return null;
  }
  return { latitude, longitude };
}

export function resolveWeatherTimezone(timezone: string): string {
  const trimmed = timezone.trim();
  return trimmed ? trimmed : 'auto';
}

export function buildMissingWeatherLocationError() {
  return {
    error: 'weather_location_required',
    message: 'Enable weather location consent and save a weather location in your profile first.',
    action: {
      label: 'Open profile',
      target: '/profile'
    }
  };
}

export function resolveUserSubject(config: AppConfig, request: FastifyRequest): string {
  if (request.auth?.subject) {
    return request.auth.subject;
  }
  if (config.auth.mode === 'noop') {
    const fromHeader = headerValue(request, 'x-user-subject');
    if (fromHeader) {
      return fromHeader;
    }
    if (config.devUserSubject) {
      return config.devUserSubject;
    }
  }
  throw new Error('missing_user_subject');
}

export function getAuthHeader(request: FastifyRequest): string | undefined {
  return headerValue(request, 'authorization');
}

export function getRequestID(request: FastifyRequest): string | undefined {
  return headerValue(request, 'x-request-id') ?? request.id;
}

export function handleGrpcRouteError(
  config: AppConfig,
  reply: { code: (statusCode: number) => { send: (body: unknown) => unknown } },
  error: unknown
) {
  if (isMissingUserSubjectError(error)) {
    return reply.code(503).send({
      error: 'missing_user_subject',
      message:
        config.auth.mode === 'noop'
          ? 'Set PULSE_PLATFORM_DEV_SUBJECT or send x-user-subject in noop mode.'
          : 'Authenticated user subject required.'
    });
  }
  if (isServiceError(error)) {
    return reply.code(mapGrpcCodeToHTTP(error.code)).send({
      error: 'upstream_grpc_error',
      message: error.details || error.message,
      grpcCode: error.code
    });
  }
  throw error;
}

function headerValue(request: FastifyRequest, key: string): string | undefined {
  const value = request.headers[key];
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

function isMissingUserSubjectError(error: unknown): boolean {
  return error instanceof Error && error.message === 'missing_user_subject';
}

function isServiceError(error: unknown): error is ServiceError {
  return typeof error === 'object' && error !== null && 'code' in error;
}

function mapGrpcCodeToHTTP(code: number): number {
  switch (code) {
    case grpcStatus.INVALID_ARGUMENT:
      return 400;
    case grpcStatus.FAILED_PRECONDITION:
      return 412;
    case grpcStatus.UNAUTHENTICATED:
      return 401;
    case grpcStatus.PERMISSION_DENIED:
      return 403;
    case grpcStatus.NOT_FOUND:
      return 404;
    case grpcStatus.ALREADY_EXISTS:
      return 409;
    case grpcStatus.DEADLINE_EXCEEDED:
      return 504;
    case grpcStatus.UNAVAILABLE:
      return 503;
    default:
      return 500;
  }
}
