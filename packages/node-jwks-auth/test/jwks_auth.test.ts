import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { afterEach, describe, expect, it } from 'vitest';
import { exportJWK, generateKeyPair, SignJWT } from 'jose';
import {
  createJwksAuthMiddleware,
  createJwksVerifier,
  extractBearerToken,
  type NextFunction,
  type NodeLikeRequest,
  type NodeLikeResponse
} from '../src/index.js';

type CapturedResponse = {
  statusCode: number;
  body: unknown;
};

function responseRecorder(): { res: NodeLikeResponse; captured: CapturedResponse } {
  const captured: CapturedResponse = { statusCode: 200, body: null };
  const res: NodeLikeResponse = {
    status(code: number) {
      captured.statusCode = code;
      return res;
    },
    json(body: unknown) {
      captured.body = body;
    }
  };
  return { res, captured };
}

async function runMiddleware(
  middleware: (req: NodeLikeRequest, res: NodeLikeResponse, next: NextFunction) => Promise<void>,
  req: NodeLikeRequest
): Promise<{ captured: CapturedResponse; nextCalled: boolean }> {
  const { res, captured } = responseRecorder();
  let nextCalled = false;
  await middleware(req, res, () => {
    nextCalled = true;
  });
  return { captured, nextCalled };
}

let closeServer: (() => Promise<void>) | null = null;

afterEach(async () => {
  if (closeServer) {
    await closeServer();
    closeServer = null;
  }
});

describe('extractBearerToken', () => {
  it('extracts token from bearer authorization header', () => {
    const token = extractBearerToken({ authorization: 'Bearer abc.def.ghi' });
    expect(token).toBe('abc.def.ghi');
  });

  it('returns empty when header is missing or malformed', () => {
    expect(extractBearerToken({})).toBe('');
    expect(extractBearerToken({ authorization: 'Basic foo' })).toBe('');
    expect(extractBearerToken({ authorization: '' })).toBe('');
  });
});

describe('createJwksAuthMiddleware', () => {
  it('rejects missing bearer token by default', async () => {
    const middleware = createJwksAuthMiddleware({
      verifier: async () => {
        throw new Error('should not verify');
      }
    });
    const { captured, nextCalled } = await runMiddleware(middleware, { headers: {} });
    expect(nextCalled).toBe(false);
    expect(captured.statusCode).toBe(401);
    expect(captured.body).toEqual({ error: 'missing_bearer_token' });
  });

  it('accepts missing token when configured', async () => {
    const middleware = createJwksAuthMiddleware({
      verifier: async () => {
        throw new Error('should not verify');
      },
      allowMissingJwt: true
    });
    const { captured, nextCalled } = await runMiddleware(middleware, { headers: {} });
    expect(nextCalled).toBe(true);
    expect(captured.statusCode).toBe(200);
  });

  it('sets request auth claims on successful verification', async () => {
    const middleware = createJwksAuthMiddleware({
      verifier: async (rawJwt: string) => ({
        subject: 'user-subject',
        email: 'dev@example.com',
        roles: ['admin'],
        rawJwt
      })
    });
    const req: NodeLikeRequest = { headers: { authorization: 'Bearer token-value' } };
    const { nextCalled } = await runMiddleware(middleware, req);
    expect(nextCalled).toBe(true);
    expect(req.auth).toBeDefined();
    expect(req.auth?.subject).toBe('user-subject');
  });
});

describe('createJwksVerifier', () => {
  it('verifies JWT against JWKS endpoint and maps claims', async () => {
    const issuer = 'https://issuer.pulse.local/realms/pulse';
    const audience = 'pulse-api';
    const { publicKey, privateKey } = await generateKeyPair('RS256');
    const jwk = await exportJWK(publicKey);
    jwk.kid = 'test-kid-1';
    jwk.alg = 'RS256';
    jwk.use = 'sig';

    const server = createServer((_: IncomingMessage, res: ServerResponse) => {
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ keys: [jwk] }));
    });
    await new Promise<void>((resolve) => {
      server.listen(0, '127.0.0.1', resolve);
    });
    closeServer = async () =>
      new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()));
      });

    const address = server.address();
    if (!address || typeof address === 'string') {
      throw new Error('jwks server address unavailable');
    }
    const jwksUrl = `http://127.0.0.1:${address.port}/jwks`;

    const token = await new SignJWT({
      email: 'dev@example.com',
      realm_access: { roles: ['viewer'] },
      resource_access: { 'pulse-api': { roles: ['admin'] } }
    })
      .setProtectedHeader({ alg: 'RS256', kid: 'test-kid-1' })
      .setIssuer(issuer)
      .setAudience(audience)
      .setSubject('kc-subject-123')
      .setIssuedAt()
      .setExpirationTime('5m')
      .sign(privateKey);

    const verifier = createJwksVerifier({
      issuerUrl: issuer,
      audience,
      jwksUrl
    });
    const claims = await verifier(token);
    expect(claims.subject).toBe('kc-subject-123');
    expect(claims.email).toBe('dev@example.com');
    expect(claims.roles).toEqual(['admin', 'viewer']);
  });
});
