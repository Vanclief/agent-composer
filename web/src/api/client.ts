type QueryValue = string | number | boolean | null | undefined;
type QueryParams = Record<string, QueryValue>;

interface ErrorEnvelope {
  message?: string;
  error?: { message?: string };
}

export function apiPath(path: string, params: QueryParams = {}) {
  const url = new URL(path, window.location.origin);

  for (const [key, value] of Object.entries(params)) {
    if (
      value !== undefined &&
      value !== null &&
      String(value).trim() !== ""
    ) {
      url.searchParams.set(key, String(value));
    }
  }

  return url.toString();
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) {
    return null;
  }

  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

function errorMessage(body: unknown, response: Response) {
  if (body && typeof body === "object") {
    const envelope = body as ErrorEnvelope;
    if (envelope.error?.message) {
      return envelope.error.message;
    }
    if (envelope.message) {
      return envelope.message;
    }
  }

  return response.statusText || `Request failed with status ${response.status}`;
}

async function requestJSON<T>(
  path: string,
  init: RequestInit = {},
  params?: QueryParams,
) {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");

  const response = await fetch(apiPath(path, params), {
    ...init,
    headers,
  });
  const body = await readBody(response);

  if (!response.ok) {
    throw new Error(errorMessage(body, response));
  }

  return body as T;
}

export function fetchJSON<T>(
  path: string,
  params?: QueryParams,
  signal?: AbortSignal,
) {
  return requestJSON<T>(path, { signal }, params);
}

export function postJSON<T>(path: string, data: unknown) {
  return requestJSON<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}
