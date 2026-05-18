// Shared Alebus API client (fetch-based).
// Intended for React Native + web dashboards.
//
// This is a thin transport wrapper only (no business logic).

/**
 * @typedef {Object} AlebusApiClientOptions
 * @property {string} baseUrl Base server URL, e.g. http://localhost:8081
 * @property {string=} apiVersion API version segment, default "v1"
 * @property {typeof fetch=} fetchImpl Custom fetch implementation (useful for RN/testing)
 * @property {Object=} defaultHeaders Additional headers to send on every request
 */

/**
 * @typedef {Object} AlebusApiError
 * @property {number} status
 * @property {string} message
 * @property {any=} body
 */

function joinUrl(baseUrl, path) {
  const trimmedBase = String(baseUrl || '').replace(/\/+$/, '');
  const trimmedPath = String(path || '').replace(/^\/+/, '');
  return `${trimmedBase}/${trimmedPath}`;
}

async function parseJsonOrText(resp) {
  const contentType = resp.headers.get('content-type') || '';
  if (contentType.includes('application/json')) {
    return resp.json();
  }
  return resp.text();
}

/**
 * Creates a client bound to /api/{version}.
 * @param {AlebusApiClientOptions} options
 */
export function createAlebusApiClient(options) {
  if (!options || !options.baseUrl) {
    throw new Error('createAlebusApiClient: baseUrl is required');
  }

  const fetchImpl = options.fetchImpl || fetch;
  const apiVersion = options.apiVersion || 'v1';
  const apiBase = joinUrl(options.baseUrl, `/api/${apiVersion}`);
  const defaultHeaders = options.defaultHeaders || {};

  async function request(method, path, body) {
    const url = joinUrl(apiBase, path);
    const headers = {
      'accept': 'application/json',
      ...defaultHeaders,
    };

    /** @type {RequestInit} */
    const init = { method, headers };

    if (body !== undefined) {
      headers['content-type'] = 'application/json';
      init.body = JSON.stringify(body);
    }

    const resp = await fetchImpl(url, init);
    const payload = await parseJsonOrText(resp);

    if (!resp.ok) {
      const payloadText = (typeof payload === 'string') ? payload.trim() : '';

      /** @type {AlebusApiError} */
      const err = {
        status: resp.status,
        message: (payload && payload.error && payload.error.message)
          ? payload.error.message
          : (payloadText ? payloadText : `HTTP ${resp.status}`),
        body: payload,
      };
      const e = new Error(err.message);
      // attach details for callers
      e.status = err.status;
      e.body = err.body;
      throw e;
    }

    return payload;
  }

  // ---- System ----
  function health() {
    // Health is unversioned on the server, but also exposed at /api/v1/health.
    return request('GET', '/health');
  }

  // ---- Routes ----
  function listRoutes() {
    return request('GET', '/routes');
  }

  // ---- Buses ----
  function listBuses() {
    return request('GET', '/buses');
  }

  // ---- Users ----
  function listUsers() {
    return request('GET', '/users');
  }

  // ---- Journeys ----
  function listJourneys() {
    return request('GET', '/journeys');
  }

  function smartPlan(params) {
    const qs = new URLSearchParams({
      originLat: String(params.originLat),
      originLon: String(params.originLon),
      destinationStopId: String(params.destinationStopId),
      radiusMeters: String(params.radiusMeters),
    });
    return request('GET', `/journeys/smart-plan?${qs.toString()}`);
  }

  function createJourney(body) {
    return request('POST', '/journeys/create', body);
  }

  return {
    apiBase,
    request,

    health,
    listRoutes,
    listBuses,
    listUsers,
    listJourneys,

    smartPlan,
    createJourney,
  };
}
