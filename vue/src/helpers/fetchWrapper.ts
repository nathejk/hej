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

// NetworkError means the request never reached the BFF — no signal, DNS failure,
// connection reset. Distinct from HttpError on purpose: an HTTP status is an answer
// from the server, this is the absence of one, and the two must not be handled
// alike. Treating a network failure as a hard error is what blanked the app offline
// (task 090), which for an app used in a forest at midnight is the worst possible
// place to be pessimistic.
export class NetworkError extends Error {
  constructor(url: string, cause?: unknown) {
    super(`Ingen forbindelse til serveren (${url})`)
    this.name = 'NetworkError'
    this.cause = cause
  }
}

async function request<T = Json>(method: string, url: string, body?: unknown): Promise<T> {
  const options: RequestInit = {
    method,
    // Send the session cookie set by the BFF on login.
    credentials: 'include',
  }

  if (body instanceof FormData) {
    // No Content-Type header on purpose: the browser has to set it, because only it
    // knows the multipart boundary. Setting `multipart/form-data` by hand produces a
    // body the server cannot parse — a classic and very confusing failure.
    options.body = body
  } else if (body !== undefined) {
    options.headers = { 'Content-Type': 'application/json' }
    options.body = JSON.stringify(body)
  }

  // fetch rejects only on a network-level failure; every HTTP status resolves. So
  // this catch means precisely "the request never reached the server".
  let response: Response
  try {
    response = await fetch(url, options)
  } catch (err) {
    throw new NetworkError(url, err)
  }

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
  /**
   * PUT a multipart body — the portrait upload (PRD 003).
   *
   * A separate entry point rather than callers passing FormData to `put`, so it is
   * obvious at the call site that this is not a JSON request.
   */
  putForm: <T = Json>(url: string, form: FormData) => request<T>('PUT', url, form),
  delete: <T = Json>(url: string) => request<T>('DELETE', url),
}
