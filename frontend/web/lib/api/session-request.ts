export function withAuthSession(
  token: string | undefined,
  init: RequestInit = {},
): RequestInit {
  const headers = new Headers(init.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);

  return {
    ...init,
    credentials: "include",
    headers,
  };
}
