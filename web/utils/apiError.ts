/**
 * Utilities for extracting actionable error information from API failures.
 *
 * The Squad Aegis backend returns errors with this shape:
 *   { message: string, code: number, data?: { error?: string, errors?: string | Record<string, string> } }
 *
 * Nuxt's useFetch surfaces this body on `error.value.data` (or
 * `error.value.response._data`). The default `error.value.message` is just the
 * HTTP status text ("Bad Request"), which is useless to users.
 *
 * These helpers normalize the various error shapes into a single, displayable
 * message plus an optional per-field error map.
 */

export interface ExtractedApiError {
  /** Best human-readable summary to show the user. */
  message: string
  /** Per-field validation errors, keyed by field name (snake_case). */
  fieldErrors?: Record<string, string>
  /** HTTP status code, if available. */
  statusCode?: number
}

interface ApiErrorBody {
  message?: string
  code?: number
  data?: {
    error?: unknown
    errors?: unknown
    [key: string]: unknown
  }
}

/**
 * Pull the API response body out of an error regardless of which fetch helper
 * raised it (useFetch's reactive error.value, $fetch's thrown FetchError, or a
 * plain Error/Response).
 */
function extractBody(err: unknown): ApiErrorBody | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as Record<string, any>

  // Nuxt useFetch error.value (FetchError shape) or imperative $fetch error.
  if (e.data && typeof e.data === 'object') return e.data as ApiErrorBody

  // Some FetchError variants nest the body under response._data.
  if (e.response && typeof e.response === 'object') {
    const resp = e.response as Record<string, any>
    if (resp._data && typeof resp._data === 'object') return resp._data as ApiErrorBody
    if (resp.data && typeof resp.data === 'object') return resp.data as ApiErrorBody
  }

  return undefined
}

function extractStatus(err: unknown): number | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as Record<string, any>
  if (typeof e.statusCode === 'number') return e.statusCode
  if (typeof e.status === 'number') return e.status
  if (e.response && typeof e.response === 'object') {
    const status = (e.response as Record<string, any>).status
    if (typeof status === 'number') return status
  }
  return undefined
}

function asFieldErrors(value: unknown): Record<string, string> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    if (typeof v === 'string' && v.trim() !== '') out[k] = v
    else if (v && typeof v === 'object' && 'message' in (v as any) && typeof (v as any).message === 'string')
      out[k] = (v as any).message
  }
  return Object.keys(out).length > 0 ? out : undefined
}

function joinFieldErrors(fieldErrors: Record<string, string>): string {
  return Object.entries(fieldErrors)
    .map(([field, msg]) => `${field}: ${msg}`)
    .join('; ')
}

/**
 * Normalize any error into { message, fieldErrors?, statusCode? } using the
 * richest information available in the API response body.
 */
export function extractApiError(
  err: unknown,
  fallback = 'An unexpected error occurred',
): ExtractedApiError {
  const body = extractBody(err)
  const statusCode = extractStatus(err)

  let fieldErrors: Record<string, string> | undefined
  let detail: string | undefined

  if (body?.data) {
    fieldErrors = asFieldErrors(body.data.errors)
    if (!fieldErrors) {
      // `errors` may itself be a string (e.g. "log_password is required for SFTP/FTP").
      if (typeof body.data.errors === 'string' && body.data.errors.trim() !== '') {
        detail = body.data.errors
      } else if (typeof body.data.error === 'string' && body.data.error.trim() !== '') {
        detail = body.data.error
      }
    }
  }

  const baseMessage =
    (typeof body?.message === 'string' && body.message.trim() !== '' && body.message) ||
    (err instanceof Error && err.message && !isHttpStatusText(err.message) && err.message) ||
    fallback

  let message = baseMessage
  if (fieldErrors) {
    message = `${baseMessage}: ${joinFieldErrors(fieldErrors)}`
  } else if (detail && detail !== baseMessage) {
    message = `${baseMessage}: ${detail}`
  }

  return { message, fieldErrors, statusCode }
}

/**
 * Some errors come back with `message` set to just "Bad Request" or
 * "Internal Server Error" — the HTTP status text. Treat those as no message so
 * the fallback or body message wins instead.
 */
function isHttpStatusText(message: string): boolean {
  const known = new Set([
    'Bad Request',
    'Unauthorized',
    'Forbidden',
    'Not Found',
    'Conflict',
    'Too Many Requests',
    'Internal Server Error',
    'Service Unavailable',
    'Gateway Timeout',
  ])
  return known.has(message.trim())
}

/**
 * Convenience: extract the message string only.
 */
export function extractApiErrorMessage(err: unknown, fallback?: string): string {
  return extractApiError(err, fallback).message
}

/**
 * Apply field-level errors from an API response onto a vee-validate form.
 * Useful for surfacing backend validation under the right field after submit.
 *
 * Pass the form returned by `useForm()` (it must expose setErrors).
 */
export function applyFieldErrorsToForm(
  form: { setErrors?: (errors: Record<string, string>) => void } | undefined | null,
  err: unknown,
): boolean {
  if (!form || typeof form.setErrors !== 'function') return false
  const { fieldErrors } = extractApiError(err)
  if (!fieldErrors) return false
  form.setErrors(fieldErrors)
  return true
}
