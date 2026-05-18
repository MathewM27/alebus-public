import type {
  BusDTO,
  CreateJourneyRequest,
  ErrorEnvelope,
  HealthResponse,
  JourneyDTO,
  LiveBusSnapshotDTO,
  NearbyStopsResponse,
  RedisStatusResponse,
  RouteDTO,
  StartJourneyResponse,
  UserDTO,
} from './types.js'

export type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export interface AlebusApiClientOptions {
  baseUrl: string
  apiVersion?: 'v1'
  fetchImpl?: FetchLike
  defaultHeaders?: Record<string, string>
}

export class AlebusApiError extends Error {
  public readonly status: number
  public readonly envelope?: ErrorEnvelope
  public readonly rawBody?: unknown

  constructor(message: string, status: number, envelope?: ErrorEnvelope, rawBody?: unknown) {
    super(message)
    this.name = 'AlebusApiError'
    this.status = status
    this.envelope = envelope
    this.rawBody = rawBody
  }
}

function joinUrl(baseUrl: string, path: string): string {
  return baseUrl.replace(/\/$/, '') + path
}

async function readJsonOrText(response: Response): Promise<unknown> {
  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    return response.json()
  }
  return response.text()
}

export function createAlebusApiClient(options: AlebusApiClientOptions) {
  const apiVersion = options.apiVersion ?? 'v1'
  const fetchImpl = options.fetchImpl ?? fetch
  const base = options.baseUrl.replace(/\/$/, '')

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const headers: Record<string, string> = {
      'content-type': 'application/json',
      ...(options.defaultHeaders ?? {}),
      ...(((init?.headers as Record<string, string>) ?? {}) as Record<string, string>),
    }

    const response = await fetchImpl(joinUrl(base, path), { ...init, headers })
    const body = await readJsonOrText(response)

    if (!response.ok) {
      const maybeEnvelope = (body && typeof body === 'object' && 'error' in (body as any)) ? (body as ErrorEnvelope) : undefined
      const msg = maybeEnvelope?.error?.message ?? `HTTP ${response.status}`
      throw new AlebusApiError(msg, response.status, maybeEnvelope, body)
    }

    return body as T
  }

  const prefix = `/api/${apiVersion}`

  return {
    request,

    health: () => request<HealthResponse>(`${prefix}/health`, { method: 'GET' }),

    listRoutes: () => request<RouteDTO[]>(`${prefix}/routes`, { method: 'GET' }),
    listBuses: () => request<BusDTO[]>(`${prefix}/buses`, { method: 'GET' }),
    listUsers: () => request<UserDTO[]>(`${prefix}/users`, { method: 'GET' }),
    listJourneys: () => request<JourneyDTO[]>(`${prefix}/journeys`, { method: 'GET' }),

    nearbyStops: (params: { lat: number; lon: number; radius?: number }) => {
      const search = new URLSearchParams({
        lat: String(params.lat),
        lon: String(params.lon),
      })
      if (params.radius != null) search.set('radius', String(params.radius))
      return request<NearbyStopsResponse>(`${prefix}/stops/nearby?${search.toString()}`, { method: 'GET' })
    },

    smartPlan: (params: { originLat: number; originLon: number; destinationStopId: string; radiusMeters: number }) => {
      const search = new URLSearchParams({
        originLat: String(params.originLat),
        originLon: String(params.originLon),
        destinationStopId: params.destinationStopId,
        radiusMeters: String(params.radiusMeters),
      })
      return request<Record<string, unknown>>(`${prefix}/journeys/smart-plan?${search.toString()}`, { method: 'GET' })
    },

    twoLegPlan: (params: { originLat: number; originLon: number; destLat: number; destLon: number; radiusMeters: number }) => {
      const search = new URLSearchParams({
        originLat: String(params.originLat),
        originLon: String(params.originLon),
        destLat: String(params.destLat),
        destLon: String(params.destLon),
        radiusMeters: String(params.radiusMeters),
      })
      return request<Record<string, unknown>>(`${prefix}/journeys/two-leg-plan?${search.toString()}`, { method: 'GET' })
    },

    createJourney: (payload: CreateJourneyRequest) =>
      request<StartJourneyResponse>(`${prefix}/journeys/create`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),

    redisStatus: () => request<RedisStatusResponse>(`${prefix}/redis/status`, { method: 'GET' }),
    redisLiveBuses: () => request<LiveBusSnapshotDTO[]>(`${prefix}/redis/buses`, { method: 'GET' }),
  }
}
