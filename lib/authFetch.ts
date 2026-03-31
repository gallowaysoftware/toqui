/**
 * Fetch wrapper that attaches a Bearer Authorization header.
 * Shared by hooks that call REST endpoints (checkout, referral, etc.).
 */
export async function authFetch(
  url: string,
  accessToken: string | null,
  options: RequestInit = {},
): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (accessToken) {
    headers["Authorization"] = `Bearer ${accessToken}`;
  }
  return fetch(url, { ...options, headers });
}
