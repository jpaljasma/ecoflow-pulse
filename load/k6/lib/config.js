export function loadConfig(env) {
  const cfg = {
    apiBaseUrl: trimTrailingSlash(env.LOADTEST_API_BASE_URL || 'http://127.0.0.1'),
    wsUrl: (env.LOADTEST_WS_URL || 'ws://127.0.0.1/ws').trim(),
    ingestUrl: (env.LOADTEST_INGEST_URL || 'http://127.0.0.1:19090/ingest').trim(),
    userSubject: (env.LOADTEST_USER_SUBJECT || 'dev-user@example.com').trim(),
    authHeader: (env.LOADTEST_AUTH_HEADER || '').trim(),
    requestTimeout: (env.LOADTEST_REQUEST_TIMEOUT || '5s').trim(),
    duration: (env.LOADTEST_DURATION || '1m').trim(),
    ingestRate: parseIntMin(env.LOADTEST_INGEST_RATE, 20, 1),
    ingestPreAllocatedVUs: parseIntMin(env.LOADTEST_INGEST_PRE_ALLOCATED_VUS, 8, 1),
    ingestMaxVUs: parseIntMin(env.LOADTEST_INGEST_MAX_VUS, 32, 1),
    queryRate: parseIntMin(env.LOADTEST_QUERY_RATE, 1, 1),
    queryPreAllocatedVUs: parseIntMin(env.LOADTEST_QUERY_PRE_ALLOCATED_VUS, 2, 1),
    queryMaxVUs: parseIntMin(env.LOADTEST_QUERY_MAX_VUS, 8, 1),
    wsVUs: parseIntMin(env.LOADTEST_WS_VUS, 20, 1),
    wsSessionTimeoutMs: parseIntMin(env.LOADTEST_WS_SESSION_TIMEOUT_MS, 4000, 250),
    wsPostTelemetryHoldMs: parseIntMin(env.LOADTEST_WS_POST_TELEMETRY_HOLD_MS, 200, 0),
    wsThinkTimeMs: parseIntMin(env.LOADTEST_WS_THINK_TIME_MS, 100, 0),
    seedCount: parseIntMin(env.LOADTEST_SEED_COUNT, 5, 1),
    queryWindowMs: parseIntMin(env.LOADTEST_QUERY_WINDOW_MS, 15 * 60 * 1000, 60 * 1000),
    queryResolution: (env.LOADTEST_QUERY_RESOLUTION || 'minute').trim(),
    ingestP95Ms: parseIntMin(env.LOADTEST_INGEST_P95_MS, 750, 1),
    queryP95Ms: parseIntMin(env.LOADTEST_QUERY_P95_MS, 1200, 1),
    wsSuccessRate: parseFloatRange(env.LOADTEST_WS_SUCCESS_RATE, 0.95, 0, 1),
    deviceID: (env.LOADTEST_DEVICE_ID || '').trim(),
    serialNumber: (env.LOADTEST_DEVICE_SERIAL_NUMBER || '').trim()
  };

  if (!['minute', 'hour', 'day'].includes(cfg.queryResolution)) {
    cfg.queryResolution = 'minute';
  }

  return cfg;
}

export function buildOptions(cfg) {
  return {
    scenarios: {
      ingest_publish: {
        executor: 'constant-arrival-rate',
        exec: 'ingestPublish',
        rate: cfg.ingestRate,
        timeUnit: '1s',
        duration: cfg.duration,
        preAllocatedVUs: cfg.ingestPreAllocatedVUs,
        maxVUs: cfg.ingestMaxVUs
      },
      history_query: {
        executor: 'constant-arrival-rate',
        exec: 'historyQuery',
        rate: cfg.queryRate,
        timeUnit: '1s',
        duration: cfg.duration,
        preAllocatedVUs: cfg.queryPreAllocatedVUs,
        maxVUs: cfg.queryMaxVUs
      },
      ws_fanout: {
        executor: 'constant-vus',
        exec: 'wsFanout',
        vus: cfg.wsVUs,
        duration: cfg.duration,
        gracefulStop: '5s'
      }
    },
    thresholds: {
      'http_req_failed{scenario:ingest_publish}': ['rate<0.01'],
      'http_req_duration{scenario:ingest_publish}': [`p(95)<${cfg.ingestP95Ms}`],
      'http_req_failed{scenario:history_query}': ['rate<0.01'],
      'http_req_duration{scenario:history_query}': [`p(95)<${cfg.queryP95Ms}`],
      ws_session_success: [`rate>${cfg.wsSuccessRate}`],
      ingest_accepted_total: ['count>0'],
      query_success_total: ['count>0']
    }
  };
}

export function buildRequestHeaders(cfg) {
  const headers = {
    Accept: 'application/json',
    'Content-Type': 'application/json'
  };
  if (cfg.userSubject) {
    headers['x-user-subject'] = cfg.userSubject;
  }
  if (cfg.authHeader) {
    headers.Authorization = cfg.authHeader;
  }
  return headers;
}

function trimTrailingSlash(value) {
  const clean = String(value || '').trim();
  return clean.endsWith('/') ? clean.slice(0, -1) : clean;
}

function parseIntMin(value, fallback, min) {
  const parsed = Number.parseInt(String(value ?? ''), 10);
  if (Number.isNaN(parsed) || parsed < min) {
    return fallback;
  }
  return parsed;
}

function parseFloatRange(value, fallback, min, max) {
  const parsed = Number.parseFloat(String(value ?? ''));
  if (Number.isNaN(parsed) || parsed < min || parsed > max) {
    return fallback;
  }
  return parsed;
}
