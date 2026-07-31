// Thin wrapper around fetch used for all API calls. Always hit relative paths
// (/api/...): in dev Vite proxies these to the `api` container; in prod the Go
// binary serves the API and SPA from the same origin. Never use bare fetch or
// axios elsewhere — go through this so auth/session and error handling stay
// consistent as the app grows.

type Json = Record<string, unknown> | unknown[] | null

// HttpError carries the HTTP status so callers (e.g. the session store) can
// treat 401 as "not authenticated" rather than a hard failure.
export class HttpError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'HttpError'
    this.status = status
  }
}

async function request<T = Json>(method: string, url: string, body?: unknown): Promise<T> {
  const options: RequestInit = {
    method,
    // Send the session cookie set by the BFF on login.
    credentials: 'include',
  }

  if (body !== undefined) {
    options.headers = { 'Content-Type': 'application/json' }
    options.body = JSON.stringify(body)
  }

  const response = await fetch(url, options)
  const text = await response.text()
  const data = text ? JSON.parse(text) : null

  if (!response.ok) {
    const message =
      data && typeof data === 'object' && 'error' in data
        ? String((data as Record<string, unknown>).error)
        : response.statusText
    throw new HttpError(response.status, message)
  }

  return data as T
}

export const fetchWrapper = {
  get: <T = Json>(url: string) => request<T>('GET', url),
  post: <T = Json>(url: string, body?: unknown) => request<T>('POST', url, body),
  put: <T = Json>(url: string, body?: unknown) => request<T>('PUT', url, body),
  delete: <T = Json>(url: string) => request<T>('DELETE', url),
}
