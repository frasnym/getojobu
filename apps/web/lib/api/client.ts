export interface FetchOptions extends RequestInit {
  token?: string | null;
}

function getBaseUrl(): string {
  // In a browser environment, use relative path so Next.js rewrites proxy requests
  // to the Go backend over same-origin, eliminating browser CORS preflight blocks.
  if (typeof window !== "undefined" && window.location?.origin) {
    return "";
  }
  return (
    process.env.NEXT_PUBLIC_API_URL ||
    process.env.NEXT_PUBLIC_BACKEND_URL ||
    "https://api.getojobu.web.id"
  ).replace(/\/+$/, "");
}

export function isMockEnabled(): boolean {
  if (typeof process !== "undefined" && process.env.NEXT_PUBLIC_ENABLE_MOCK === "true") {
    return true;
  }
  return false;
}

export async function apiClient<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const { token, headers: customHeaders, ...rest } = options;

  const headers = new Headers(customHeaders);
  headers.set("Content-Type", "application/json");

  let authToken = token;
  // If no token was explicitly passed, attempt to retrieve from global Clerk session
  if (!authToken && typeof window !== "undefined") {
    try {
      const clerk = (window as any).Clerk;
      if (clerk?.session) {
        authToken = await clerk.session.getToken();
      }
    } catch (e) {
      console.warn("Could not retrieve Clerk token from window.Clerk:", e);
    }
  }

  if (authToken) {
    headers.set("Authorization", `Bearer ${authToken}`);
  }

  const baseUrl = getBaseUrl();
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const url = `${baseUrl}${normalizedPath}`;

  const response = await fetch(url, {
    ...rest,
    headers,
  });

  if (!response.ok) {
    const errorBody = await response.json().catch(() => ({}));
    const message =
      errorBody?.error?.message ||
      errorBody?.message ||
      `API request failed with status ${response.status}`;
    const error = new Error(message);
    (error as any).status = response.status;
    (error as any).code = errorBody?.error?.code || errorBody?.code;
    (error as any).details = errorBody?.error?.details || errorBody?.details;
    throw error;
  }

  const body = await response.json();
  if (body && typeof body === "object") {
    if (body.success === false && body.error) {
      const message = body.error.message || "API request returned an error";
      const error = new Error(message);
      (error as any).status = response.status;
      (error as any).code = body.error.code;
      (error as any).details = body.error.details;
      throw error;
    }
    if ("data" in body && "success" in body) {
      return body.data as T;
    }
  }

  return body as T;
}
