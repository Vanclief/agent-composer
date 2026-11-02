// The agc API, same origin under /api. Errors become ApiError whose message
// is the backend's text, verbatim: show error.message, never rephrase it.
import createClient from 'openapi-fetch'

import type { components, paths } from './schema'

// The environment block of a sandbox (ADR 0003, unchanged by ADR 0008).
// Tasks and sandboxes themselves are typed in ./tasks.
export type TaskSpec = components['schemas']['TaskSpec']
export type SavedSpec = components['schemas']['SavedSpec']
export type Secret = components['schemas']['Secret']

export class ApiError extends Error {
  // HTTP status, 0 when the request never got an answer.
  readonly status: number
  // Backend error code such as EINVALID or ENOTFOUND, when it sent one.
  readonly code?: string
  readonly requestId?: string

  constructor(status: number, message: string, code?: string, requestId?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.requestId = requestId
  }
}

// Builds an ApiError from a failed response body. Tasks, specs and secrets
// answer {"error":{"code","message","request_id"}}, messaging answers
// {"error":"..."}, and echo's own errors (unknown route) {"message":"..."}.
export function toApiError(response: Response, body: unknown): ApiError {
  if (typeof body === 'object' && body !== null) {
    const { error, message } = body as { error?: unknown; message?: unknown }
    if (typeof error === 'object' && error !== null) {
      const e = error as { code?: string; message?: string; request_id?: string }
      if (e.message) {
        return new ApiError(response.status, e.message, e.code, e.request_id)
      }
    }
    if (typeof error === 'string' && error) {
      return new ApiError(response.status, error)
    }
    if (typeof message === 'string' && message) {
      return new ApiError(response.status, message)
    }
  }
  if (typeof body === 'string' && body.trim()) {
    return new ApiError(response.status, body.trim())
  }
  return new ApiError(response.status, `${response.status} ${response.statusText || 'request failed'}`.trim())
}

// Typed client for every route in docs/openapi.yaml.
export const api = createClient<paths>({ baseUrl: '/api' })

// unwrap turns an openapi-fetch result into its data or a thrown ApiError.
//
//   const task = await unwrap(api.GET('/tasks/{id}', { params: { path: { id } } }))
export async function unwrap<T>(request: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await request
  if (error !== undefined || !response.ok) {
    throw toApiError(response, error)
  }
  return data as T
}

// apiFetch is for the routes openapi.yaml does not describe (terminals,
// messaging). path is relative to /api, e.g. "/msg/channels".
export async function apiFetch<T>(
  method: 'GET' | 'POST' | 'PUT' | 'DELETE',
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const init: RequestInit = { method, signal }
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }
  const response = await fetch('/api' + path, init)
  const text = await response.text()
  let parsed: unknown = text
  try {
    parsed = text ? JSON.parse(text) : undefined
  } catch {
    // Not JSON: keep the text, toApiError shows it as is.
  }
  if (!response.ok) {
    throw toApiError(response, parsed)
  }
  return parsed as T
}

// errorMessage is what to print for any thrown value.
export function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  return String(error)
}
