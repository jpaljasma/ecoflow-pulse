import type { FastifyReply, FastifyRequest, preHandlerHookHandler } from 'fastify';

import { createJwksVerifier, extractBearerToken, type AuthClaims, type JwtVerifier } from '@ecoflow-pulse/node-jwks-auth';

import type { AppConfig } from './config.js';

declare module 'fastify' {
  interface FastifyRequest {
    auth?: AuthClaims;
  }
}

export function buildAuthPreHandler(config: AppConfig): preHandlerHookHandler {
  if (config.auth.mode === 'noop') {
    return async () => {};
  }
  const verifier = createJwksVerifier({
    issuerUrl: config.auth.issuerUrl,
    audience: config.auth.audience,
    jwksUrl: config.auth.jwksUrl,
    allowMissingJwt: config.auth.allowMissingJwt
  });
  return makeVerifierPreHandler(verifier, config.auth.allowMissingJwt);
}

export function makeVerifierPreHandler(
  verifier: JwtVerifier,
  allowMissingJwt: boolean
): preHandlerHookHandler {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    const rawJwt = extractBearerToken(request.headers as Record<string, string | string[] | undefined>);
    if (!rawJwt) {
      if (allowMissingJwt) {
        return;
      }
      void reply.code(401).send({ error: 'missing_bearer_token' });
      return;
    }
    try {
      request.auth = await verifier(rawJwt);
    } catch {
      void reply.code(401).send({ error: 'invalid_bearer_token' });
    }
  };
}
