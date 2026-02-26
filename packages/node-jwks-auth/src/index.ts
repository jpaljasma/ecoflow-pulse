import { createRemoteJWKSet, errors as joseErrors, jwtVerify, type JWTPayload } from 'jose';

export type AuthClaims = {
  subject: string;
  email?: string;
  roles: string[];
  rawJwt: string;
};

export type JwtVerifier = (rawJwt: string) => Promise<AuthClaims>;

export type NodeLikeRequest = {
  headers: Record<string, string | string[] | undefined>;
  auth?: AuthClaims;
};

export type NodeLikeResponse = {
  status: (code: number) => NodeLikeResponse;
  json: (body: unknown) => void;
};

export type NextFunction = (err?: unknown) => void;

export type JwksVerifierConfig = {
  issuerUrl: string;
  audience: string;
  jwksUrl?: string;
  allowMissingJwt?: boolean;
};

export type JwtMiddlewareConfig = {
  verifier: JwtVerifier;
  allowMissingJwt?: boolean;
};

export function createJwksVerifier(cfg: JwksVerifierConfig): JwtVerifier {
  const issuerUrl = cfg.issuerUrl.trim();
  const audience = cfg.audience.trim();
  if (!issuerUrl) {
    throw new Error('issuerUrl is required');
  }
  if (!audience) {
    throw new Error('audience is required');
  }
  const issuerBaseUrl = trimTrailingSlashes(issuerUrl);
  const jwksUrl = (cfg.jwksUrl ?? `${issuerBaseUrl}/protocol/openid-connect/certs`).trim();
  const jwks = createRemoteJWKSet(new URL(jwksUrl));
  return async (rawJwt: string): Promise<AuthClaims> => {
    const { payload } = await jwtVerify(rawJwt, jwks, {
      issuer: issuerUrl,
      audience
    });
    const subject = payload.sub?.trim();
    if (!subject) {
      throw new Error('jwt subject is required');
    }
    return {
      subject,
      email: typeof payload.email === 'string' ? payload.email : undefined,
      roles: extractRoles(payload),
      rawJwt
    };
  };
}

function trimTrailingSlashes(value: string): string {
  let end = value.length;
  while (end > 0 && value.charCodeAt(end - 1) === 47) {
    end -= 1;
  }
  return end === value.length ? value : value.slice(0, end);
}

export function createJwksAuthMiddleware(cfg: JwtMiddlewareConfig) {
  const verifier = cfg.verifier;
  const allowMissingJwt = cfg.allowMissingJwt === true;
  return async (req: NodeLikeRequest, res: NodeLikeResponse, next: NextFunction): Promise<void> => {
    try {
      const rawJwt = extractBearerToken(req.headers);
      if (!rawJwt) {
        if (allowMissingJwt) {
          next();
          return;
        }
        res.status(401).json({ error: 'missing_bearer_token' });
        return;
      }
      req.auth = await verifier(rawJwt);
      next();
    } catch (err) {
      const isJoseError = err instanceof joseErrors.JOSEError;
      res.status(401).json({
        error: isJoseError ? 'invalid_bearer_token' : 'auth_verification_failed'
      });
    }
  };
}

export function extractBearerToken(headers: Record<string, string | string[] | undefined>): string {
  const raw = headers.authorization;
  if (!raw) return '';
  const value = Array.isArray(raw) ? raw[0] : raw;
  if (!value) return '';
  const trimmed = value.trim();
  if (!trimmed.toLowerCase().startsWith('bearer ')) return '';
  return trimmed.slice(7).trim();
}

function extractRoles(payload: JWTPayload): string[] {
  const out = new Set<string>();
  const realm = (payload as Record<string, unknown>).realm_access as Record<string, unknown> | undefined;
  if (realm && Array.isArray(realm.roles)) {
    for (const role of realm.roles) {
      if (typeof role === 'string' && role.trim()) {
        out.add(role.trim());
      }
    }
  }
  const resources = (payload as Record<string, unknown>).resource_access as
    | Record<string, { roles?: unknown }>
    | undefined;
  if (resources) {
    for (const value of Object.values(resources)) {
      if (!value || !Array.isArray(value.roles)) continue;
      for (const role of value.roles) {
        if (typeof role === 'string' && role.trim()) {
          out.add(role.trim());
        }
      }
    }
  }
  return [...out.values()].sort();
}
