type QueryValue = string | number | boolean | null | undefined;
type QueryParams = Record<string, QueryValue>;

interface ErrorEnvelope {
  message?: string;
  error?: string | { message?: string };
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

function unwrap<T>(body: unknown): T {
  if (body && typeof body === "object") {
    const envelope = body as Record<string, unknown>;
    if (envelope.data && typeof envelope.data === "object") {
      return envelope.data as T;
    }
    if (envelope.response && typeof envelope.response === "object") {
      return envelope.response as T;
    }
  }

  return body as T;
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
    if (envelope.message) {
      return envelope.message;
    }
    if (typeof envelope.error === "string") {
      return envelope.error;
    }
    if (envelope.error?.message) {
      return envelope.error.message;
    }
  }

  if (typeof body === "string" && body.trim()) {
    return body;
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

  return unwrap<T>(body);
}

export function fetchJSON<T>(path: string, params?: QueryParams) {
  return requestJSON<T>(path, {}, params);
}

export function postJSON<T>(path: string, data: unknown) {
  return requestJSON<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export function putJSON<T>(path: string, data: unknown) {
  return requestJSON<T>(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export function deleteJSON<T>(path: string) {
  return requestJSON<T>(path, { method: "DELETE" });
}
