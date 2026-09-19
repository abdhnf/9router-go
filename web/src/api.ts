// Shared fetch wrapper for the dashboard.
//
// The console authenticates with an HttpOnly session cookie, so no credential
// is ever read from or written to localStorage. `credentials: 'include'` is the
// part that actually sends the cookie on same-origin XHR/fetch.
//
// A 401 means the session expired or was revoked server-side (engine restart,
// logout elsewhere, cookie cleared). We broadcast that once so the top-level App
// can drop to the login screen instead of leaving every panel silently empty.

export const SESSION_EXPIRED_EVENT = '9router:session-expired'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function apiFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const res = await fetch(input, { ...init, credentials: 'include' })
  if (res.status === 401 || res.status === 403) {
    window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT))
  }
  return res
}

/** JSON request helper that throws ApiError with the engine's message. */
export async function apiJSON<T = any>(input: string, init: RequestInit = {}): Promise<T> {
  const res = await apiFetch(input, init)
  if (!res.ok) {
    let message = `HTTP ${res.status}`
    try {
      const body = await res.json()
      message = body?.error?.message || body?.message || message
    } catch {
      /* non-JSON error body — keep the status line */
    }
    throw new ApiError(res.status, message)
  }
  return res.json() as Promise<T>
}

/** POST/PUT/PATCH with a JSON body and the right Content-Type. */
export function apiSend<T = any>(
  input: string,
  method: 'POST' | 'PUT' | 'PATCH' | 'DELETE',
  body?: unknown
): Promise<T> {
  return apiJSON<T>(input, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}
