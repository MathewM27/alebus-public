export interface SSEMessageEvent {
  data: string
}

export interface SSEEventSource {
  addEventListener(type: string, listener: (ev: SSEMessageEvent) => void): void
  removeEventListener(type: string, listener: (ev: SSEMessageEvent) => void): void
  close(): void
  onopen: ((ev: unknown) => void) | null
  onerror: ((ev: unknown) => void) | null
}

export interface SSEEventSourceFactory {
  (url: string, init?: { withCredentials?: boolean }): SSEEventSource
}

export interface SSEReconnectOptions {
  enabled?: boolean
  baseDelayMs?: number
  maxDelayMs?: number
  jitterRatio?: number
}

export type SSEConnectionState = 'connecting' | 'open' | 'closed'

export interface ConnectSSEOptions {
  url: string

  /**
   * Optional injection point.
   * - Browser: defaults to globalThis.EventSource
   * - React Native: pass a polyfilled EventSource constructor
   */
  eventSourceFactory?: SSEEventSourceFactory

  withCredentials?: boolean

  reconnect?: SSEReconnectOptions

  /** Called for every named SSE event (event: <name>) */
  onEvent?: (eventName: string, data: string) => void

  /** Convenience hooks for the canonical Alebus stream events */
  onReady?: (ev: SSEReadyEvent) => void
  onPing?: (ev: Record<string, unknown>) => void
  onBusUpdate?: (ev: SSEBusUpdateEvent<unknown>) => void

  onOpen?: () => void
  onError?: (err: unknown) => void
  onStateChange?: (state: SSEConnectionState) => void
}

export interface SSEConnection {
  close(): void
  updateUrl(url: string): void
  state(): SSEConnectionState
}

export interface SSEReadyEvent {
  serverTs: string
  stream?: string
}

export interface SSEBusUpdateEvent<TBus> {
  serverTs: string
  seq?: number
  bus: TBus
}

function defaultEventSourceFactory(url: string, init?: { withCredentials?: boolean }): SSEEventSource {
  const ES = (globalThis as any).EventSource as undefined | (new (url: string, init?: any) => SSEEventSource)
  if (!ES) {
    throw new Error(
      'EventSource is not available in this runtime. ' +
        'Provide connectSSE({ eventSourceFactory }) (e.g. eventsource polyfill for React Native).',
    )
  }

  return new ES(url, init)
}

function clamp(n: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, n))
}

function nextDelayMs(attempt: number, baseDelayMs: number, maxDelayMs: number, jitterRatio: number): number {
  const exp = baseDelayMs * Math.pow(2, Math.max(0, attempt))
  const capped = Math.min(maxDelayMs, exp)
  const jitter = capped * clamp(jitterRatio, 0, 1) * (Math.random() * 2 - 1)
  return Math.round(Math.max(0, capped + jitter))
}

function safeParseJson(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

export function connectSSE(options: ConnectSSEOptions): SSEConnection {
  const reconnectEnabled = options.reconnect?.enabled ?? true
  const baseDelayMs = options.reconnect?.baseDelayMs ?? 500
  const maxDelayMs = options.reconnect?.maxDelayMs ?? 30_000
  const jitterRatio = options.reconnect?.jitterRatio ?? 0.2

  let currentUrl = options.url
  let es: SSEEventSource | undefined
  let state: SSEConnectionState = 'connecting'
  let closed = false
  let attempt = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined

  function setState(next: SSEConnectionState) {
    state = next
    options.onStateChange?.(next)
  }

  function cleanupEventSource() {
    if (!es) return
    try {
      es.close()
    } catch {
      // ignore
    }
    es = undefined
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = undefined
    }
  }

  function scheduleReconnect() {
    if (closed || !reconnectEnabled) return
    clearReconnectTimer()

    const delay = nextDelayMs(attempt, baseDelayMs, maxDelayMs, jitterRatio)
    attempt++
    reconnectTimer = setTimeout(() => {
      if (closed) return
      start()
    }, delay)
  }

  const listeners = new Map<string, (ev: SSEMessageEvent) => void>()

  function attachListener(eventName: string, handler: (ev: SSEMessageEvent) => void) {
    listeners.set(eventName, handler)
    es?.addEventListener(eventName, handler)
  }

  function detachAllListeners() {
    if (!es) return
    for (const [name, handler] of listeners) {
      es.removeEventListener(name, handler)
    }
    listeners.clear()
  }

  function start() {
    setState('connecting')
    cleanupEventSource()

    const factory = options.eventSourceFactory ?? defaultEventSourceFactory
    es = factory(currentUrl, { withCredentials: options.withCredentials })

    // Open
    es.onopen = () => {
      attempt = 0
      setState('open')
      options.onOpen?.()
    }

    // Error
    es.onerror = (ev) => {
      // We intentionally close + manage our own backoff instead of relying on
      // EventSource's internal retry behavior, so the app has predictable control.
      options.onError?.(ev)

      if (closed) return

      try {
        detachAllListeners()
      } finally {
        cleanupEventSource()
      }

      setState('connecting')
      scheduleReconnect()
    }

    // Generic named events
    options.onEvent &&
      attachListener('message', (evt) => {
        options.onEvent?.('message', evt.data)
      })

    attachListener('ready', (evt) => {
      options.onEvent?.('ready', evt.data)
      const parsed = safeParseJson(evt.data)
      if (parsed && typeof parsed === 'object') {
        options.onReady?.(parsed as SSEReadyEvent)
      }
    })

    attachListener('ping', (evt) => {
      options.onEvent?.('ping', evt.data)
      const parsed = safeParseJson(evt.data)
      if (parsed && typeof parsed === 'object') {
        options.onPing?.(parsed as Record<string, unknown>)
      }
    })

    attachListener('bus.update', (evt) => {
      options.onEvent?.('bus.update', evt.data)
      const parsed = safeParseJson(evt.data)
      if (parsed && typeof parsed === 'object') {
        options.onBusUpdate?.(parsed as SSEBusUpdateEvent<unknown>)
      }
    })
  }

  // Initial connect
  start()

  return {
    close() {
      if (closed) return
      closed = true
      clearReconnectTimer()
      if (es) {
        detachAllListeners()
        cleanupEventSource()
      }
      setState('closed')
    },

    updateUrl(url: string) {
      currentUrl = url
      if (closed) return
      clearReconnectTimer()
      if (es) {
        detachAllListeners()
        cleanupEventSource()
      }
      attempt = 0
      start()
    },

    state() {
      return state
    },
  }
}

export function buildStreamUrl(params: {
  baseUrl: string
  apiVersion?: 'v1'
  path: '/stream/live-buses' | '/stream/journeys'
  query?: Record<string, string | number | boolean | undefined>
}): string {
  const apiVersion = params.apiVersion ?? 'v1'
  const base = params.baseUrl.replace(/\/$/, '')
  const url = new URL(`${base}/api/${apiVersion}${params.path}`)

  for (const [k, v] of Object.entries(params.query ?? {})) {
    if (v === undefined) continue
    url.searchParams.set(k, String(v))
  }

  return url.toString()
}

export function connectLiveBusesStream(options: Omit<ConnectSSEOptions, 'url'> & {
  baseUrl: string
  apiVersion?: 'v1'
  routeId?: string
  busId?: string
}): SSEConnection {
  const url = buildStreamUrl({
    baseUrl: options.baseUrl,
    apiVersion: options.apiVersion,
    path: '/stream/live-buses',
    query: {
      routeId: options.routeId,
      busId: options.busId,
    },
  })

  return connectSSE({ ...options, url })
}

export function connectJourneyStream(options: Omit<ConnectSSEOptions, 'url'> & {
  baseUrl: string
  apiVersion?: 'v1'
  journeyId: string
  busIds?: string[]
}): SSEConnection {
  const busIdsCsv = options.busIds && options.busIds.length > 0 ? options.busIds.join(',') : undefined

  const url = buildStreamUrl({
    baseUrl: options.baseUrl,
    apiVersion: options.apiVersion,
    path: '/stream/journeys',
    query: {
      journeyId: options.journeyId,
      busIds: busIdsCsv,
    },
  })

  return connectSSE({ ...options, url })
}
