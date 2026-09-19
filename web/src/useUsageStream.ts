import { useEffect, useState } from 'react'

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
 * Uses fetch + ReadableStream rather than EventSource on purpose: the endpoint
 * sits behind RequireApiKey, and EventSource cannot attach an Authorization
 * header. With EventSource the request always answers 401 and every telemetry
 * card silently stays at zero. Passing the key via `?key=` is not an option —
 * ExtractApiKey deliberately rejects query-string keys because they leak
 * through browser history, referrers, and upstream proxy logs.
 */
export function useUsageStream(apiKey: string, enabled: boolean) {
  const [metrics, setMetrics] = useState<UsageMetrics>(INITIAL_METRICS)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!enabled || !apiKey) return

    const controller = new AbortController()
    let reconnectTimer: number | undefined
    let disposed = false

    // One SSE frame: join every `data:` line, ignore `: ping` keep-alives.
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
          headers: { Authorization: `Bearer ${apiKey}` },
          signal: controller.signal,
        })
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
  }, [apiKey, enabled])

  return { metrics, connected, error }
}
