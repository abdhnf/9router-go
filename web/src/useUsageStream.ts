import { useEffect, useState } from 'react'
import { SESSION_EXPIRED_EVENT } from './api'

export interface UsageMetrics {
  activeRequests: number
  recentRequests: number
  errorProvider: string
  pending: {
    total: number
    byAccount?: Record<string, Record<string, number>>
  }
}

const INITIAL_METRICS: UsageMetrics = {
  activeRequests: 0,
  recentRequests: 0,
  errorProvider: '',
  pending: { total: 0 },
}

const RECONNECT_DELAY_MS = 3000

/**
 * Streams `/usage/stream` into React state.
 *
 * Uses fetch + ReadableStream rather than EventSource on purpose: EventSource
 * cannot attach an Authorization header, and passing the key via `?key=` is not
 * an option because ExtractApiKey deliberately rejects query-string keys (they
 * leak through browser history, referrers, and upstream proxy logs).
 *
 * Auth now rides on the session cookie, so `credentials: 'include'` replaces the
 * bearer header. A 401 means the session died (engine restart, logout, expired
 * cookie): retrying cannot fix it and would hammer the engine every few seconds,
 * so we stop and let the App drop to the login screen.
 */
export function useUsageStream(enabled: boolean) {
  const [metrics, setMetrics] = useState<UsageMetrics>(INITIAL_METRICS)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!enabled) return

    const controller = new AbortController()
    let reconnectTimer: number | undefined
    let disposed = false

    // One SSE frame: join every `data:` lines, ignore `: ping` keep-alives.
    const dispatch = (frame: string) => {
      const payload = frame
        .split('\n')
        .filter((line) => line.startsWith('data:'))
        .map((line) => line.slice(5).trimStart())
        .join('\n')
      if (!payload) return
      try {
        setMetrics(JSON.parse(payload) as UsageMetrics)
      } catch {
        // ignore malformed frame, keep the stream alive
      }
    }

    const pump = async () => {
      try {
        const res = await fetch('/usage/stream', {
          credentials: 'include',
          signal: controller.signal,
        })

        if (res.status === 401 || res.status === 403) {
          setConnected(false)
          setError('unauthorized')
          window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT))
          return
        }
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        if (!res.body) throw new Error('Response body tidak tersedia')

        setConnected(true)
        setError('')

        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''

        for (;;) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })

          let separator = buffer.indexOf('\n\n')
          while (separator !== -1) {
            dispatch(buffer.slice(0, separator))
            buffer = buffer.slice(separator + 2)
            separator = buffer.indexOf('\n\n')
          }
        }
      } catch (err) {
        if (controller.signal.aborted) return
        setError(err instanceof Error ? err.message : 'Stream terputus')
      }

      setConnected(false)
      if (!disposed) {
        reconnectTimer = window.setTimeout(pump, RECONNECT_DELAY_MS)
      }
    }

    pump()

    return () => {
      disposed = true
      controller.abort()
      if (reconnectTimer) window.clearTimeout(reconnectTimer)
      setConnected(false)
    }
  }, [enabled])

  return { metrics, connected, error }
}
