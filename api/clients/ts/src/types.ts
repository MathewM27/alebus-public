export type ApiAudience = 'Public' | 'Operator' | 'Admin'

export interface APIError {
  code: string
  message: string
  details?: Record<string, unknown>
  requestId?: string
}

export interface ErrorEnvelope {
  error: APIError
}

export interface HealthResponse {
  status: string
  database: string
}

export interface StopDTO {
  id: string
  name: string
  lat?: number
  lon?: number
  cumulative_distance?: number
  location?: { latitude?: number; longitude?: number }
}

export interface RouteDTO {
  routeId: string
  name: string
  direction: string
  routeType: string
  status: string
  stops: StopDTO[]
  activeFrom?: string
  activeUntil?: string
  createdAt?: string
}

export interface BusPositionDTO {
  lat: number
  lon: number
  timestamp: string
  accuracy: number
  speedKmh: number
}

export interface BusDTO {
  busId: string
  operatorId: string
  routeId: string
  direction: string
  stopIndex: number
  currentSpeed: number
  isAtTerminal: boolean
  terminalArrivalTime?: string
  position: BusPositionDTO
  status: string
  updatedAt: string
}

export interface UserDTO {
  userId: string
  email: string
  subscription: {
    status: number
    plan: number
    startDate: string
    expiryDate: string
  }
  savedLocations: Array<{
    id: string
    name: string
    lat: number
    lon: number
    type: number
    createdAt: string
  }>
  createdAt: string
}

// NOTE: This DTO intentionally matches JSON keys produced by the Go layer.
export interface JourneyBusRecommendationDTO {
  bus_id: string
  operator_id: string
  estimated_arrival: number
  distance_to_origin_stop: number
  direction: number
  required_direction: number
  is_wrong_direction: boolean
  confidence_level: number
  display_text: string
  ranking_score: number
  actual_route_distance: number
}

export interface JourneyDTO {
  journeyId: string
  userId: string
  status: number
  phase: number
  originStopId: string
  destinationStopId: string
  requiredDirection: number
  activeBusId?: string
  recommendations: JourneyBusRecommendationDTO[]
  startedAt: string
  updatedAt: string
}

export interface CreateJourneyRequest {
  journeyId?: string
  userId: string
  originLat: number
  originLon: number
  originStopId?: string
  destinationStopId: string
  radiusMeters?: number
}

export interface StartJourneyResponse {
  journeyId: string
  originStopId: string
  destinationStopId: string
  requiredDirection: number
  recommendations: JourneyBusRecommendationDTO[]
  estimatedDurationMs: number
}

export interface NearbyStopsResponse {
  stops?: Array<{ stopId: string; name: string; index: number; distanceMeters: number }>
}

export interface RedisStatusResponse {
  connected: boolean
  enabled: boolean
  url: string
  liveBusCount: number
  geoIndexCount: number
  error?: string
}

export interface LiveBusSnapshotDTO {
  busId: string
  routeId: string
  lat: number
  lon: number
  speedKmh: number
  heading: number
  direction: string
  stopIndex: number
  lastUpdate: string
  ttlRemaining: number
}
