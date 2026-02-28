import type { FastifyReply, FastifyRequest, preValidationHookHandler } from 'fastify';

import {
  createJwksVerifier,
  extractBearerToken,
  type AuthClaims,
  type JwtVerifier
} from '@ecoflow-pulse/node-jwks-auth';

import type { AppConfig } from './config.js';

declare module 'fastify' {
  interface FastifyRequest {
    auth?: AuthClaims;
    wsAuthHeader?: string;
  }
}

export function buildWsPreValidation(config: AppConfig): preValidationHookHandler {
  if (config.auth.mode === 'noop') {
    return async () => {};
  }
  const verifier = createJwksVerifier({
    issuerUrl: config.auth.issuerUrl,
    audience: config.auth.audience,
    jwksUrl: config.auth.jwksUrl,
    allowMissingJwt: config.auth.allowMissingJwt
  });
  return makeWsVerifierPreValidation(verifier, config.auth.allowMissingJwt);
}

export function makeWsVerifierPreValidation(
  verifier: JwtVerifier,
  allowMissingJwt: boolean
): preValidationHookHandler {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    const rawJwt = extractWsToken(request);
    if (!rawJwt) {
      if (allowMissingJwt) {
        return;
      }
      void reply.code(401).send({ error: 'missing_bearer_token' });
      return;
    }
    try {
      request.auth = await verifier(rawJwt);
      request.wsAuthHeader = `Bearer ${rawJwt}`;
    } catch {
      void reply.code(401).send({ error: 'invalid_bearer_token' });
    }
  };
}

function extractWsToken(request: FastifyRequest): string {
  const fromQuery = extractTokenFromUrl(request.raw.url);
  if (fromQuery) {
    return fromQuery;
  }
  return extractBearerToken(request.headers as Record<string, string | string[] | undefined>);
}

function extractTokenFromUrl(rawUrl: string | undefined): string {
  const url = new URL(rawUrl ?? '/', 'http://localhost');
  return url.searchParams.get('token')?.trim() ?? '';
}
