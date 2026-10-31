/**
 * src/utils/api.ts
 *
 * Typed wrapper around the native fetch API.
 * - Automatically prefixes requests with the API base URL.
 * - Sends cookies (JWT) with every request (`credentials: "include"`).
 * - Sets JSON headers and stringifies request bodies.
 * - Parses JSON responses and returns typed data.
 * - Centralises error handling via the ApiError class.
 *
 * Usage examples:
 *
 *   const invoices = await api.get<Invoice[]>('/api/v1/invoices');
 *   const newInv = await api.post<Invoice>('/api/v1/invoices', { fileUrl: ... });
 *
 * The wrapper works both on the client and during SSR in Next.js.
 */

type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export class ApiError extends Error {
  /** HTTP status code returned by the server */
  readonly status: number;
  /** Optional payload returned by the server (often an error object) */
  readonly data?: unknown;

  constructor(message: string, status: number, data?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.data = data;
    // Set the prototype explicitly (required when targeting ES5)
    Object.setPrototypeOf(this, ApiError.prototype);
  }
}

/**
 * Build the absolute URL for a request.
 * If the supplied path is already absolute (starts with http/https) it is returned unchanged.
 * Otherwise it is prefixed with the API base URL defined in NEXT_PUBLIC_API_URL.
 */
function buildUrl(path: string, query?: Record<string, any>): string {
  const base = process.env.NEXT_PUBLIC_API_URL?.replace(/\/+$/, '') ?? '';
  const url = path.startsWith('http://') || path.startsWith('https://')
    ? new URL(path)
    : new URL(`${base}${path.startsWith('/') ? '' : '/'}${path}`, base);

  if (query) {
    Object.entries(query).forEach(([key, value]) => {
      if (value === undefined || value === null) return;
      if (Array.isArray(value)) {
        value.forEach(v => url.searchParams.append(key, String(v)));
      } else {
        url.searchParams.append(key, String(value));
      }
    });
  }

  return url.toString();
}

/**
 * Centralised response handling.
 * - Parses JSON when the `Content-Type` header indicates JSON.
 * - Throws an ApiError for non‑2xx responses, attaching the parsed payload if available.
 */
async function handleResponse<T>(response: Response): Promise<T> {
  const contentType = response.headers.get('content-type') ?? '';

  // Helper to safely parse JSON (may fail if body is empty or malformed)
  const parseJson = async (): Promise<unknown> => {
    try {
      return await response.json();
    } catch {
      return undefined;
    }
  };

  if (response.ok) {
    if (contentType.includes('application/json')) {
      return (await response.json()) as T;
    }
    // For non‑JSON successful responses (e.g., 204 No Content) return undefined
    return undefined as unknown as T;
  }

  // Error path – try to extract a useful payload
  const errorPayload = await parseJson();
  const message =
    (errorPayload && typeof errorPayload === 'object' && 'message' in errorPayload
      ? (errorPayload as any).message
      : response.statusText) || 'Unexpected error';

  throw new ApiError(message, response.status, errorPayload);
}

/**
 * Core request implementation.
 */
async function request<T>(
  method: HttpMethod,
  path: string,
  options: {
    /** Body will be JSON‑stringified unless `raw` is true */
    body?: any;
    /** Set to true when sending already‑stringified or binary payloads */
    raw?: boolean;
    /** Additional request init options (e.g., signal for abort) */
    init?: RequestInit;
    /** Query parameters appended to the URL */
    query?: Record<string, any>;
  } = {}
): Promise<T> {
  const { body, raw = false, init = {}, query } = options;

  const url = buildUrl(path, query);

  const headers: HeadersInit = {
    Accept: 'application/json',
    ...init.headers,
  };

  const fetchInit: RequestInit = {
    method,
    credentials: 'include', // send JWT cookie automatically
    headers,
    ...init,
  };

  if (body !== undefined) {
    if (!raw) {
      headers['Content-Type'] = 'application/json';
      fetchInit.body = JSON.stringify(body);
    } else {
      // raw payload – caller is responsible for appropriate headers
      fetchInit.body = body;
    }
  }

  const response = await fetch(url, fetchInit);
  return handleResponse<T>(response);
}

/* -------------------------------------------------------------------------- */
/* Convenience HTTP verb helpers                                            */
/* -------------------------------------------------------------------------- */

function get<T>(path: string, opts?: { query?: Record<string, any>; init?: RequestInit }): Promise<T> {
  return request<T>('GET', path, { query: opts?.query, init: opts?.init });
}

function del<T>(path: string, opts?: { query?: Record<string, any>; init?: RequestInit }): Promise<T> {
  return request<T>('DELETE', path, { query: opts?.query, init: opts?.init });
}

function post<T>(path: string, body: any, opts?: { raw?: boolean; init?: RequestInit; query?: Record<string, any> }): Promise<T> {
  return request<T>('POST', path, { body, raw: opts?.raw, init: opts?.init, query: opts?.query });
}

function put<T>(path: string, body: any, opts?: { raw?: boolean; init?: RequestInit; query?: Record<string, any> }): Promise<T> {
  return request<T>('PUT', path, { body, raw: opts?.raw, init: opts?.init, query: opts?.query });
}

function patch<T>(path: string, body: any, opts?: { raw?: boolean; init?: RequestInit; query?: Record<string, any> }): Promise<T> {
  return request<T>('PATCH', path, { body, raw: opts?.raw, init: opts?.init, query: opts?.query });
}

/* -------------------------------------------------------------------------- */
/* Exported API object                                                       */
/* -------------------------------------------------------------------------- */

const api = {
  get,
  post,
  put,
  patch,
  delete: del,
  request,
  ApiError,
};

export default api;