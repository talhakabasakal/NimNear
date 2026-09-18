const STATUS_MESSAGES: Record<number, string> = {
  401: "Sign in to continue.",
  403: "You do not have permission to do that.",
  404: "This item could not be found.",
  409: "This action conflicts with the current state. Refresh and try again.",
  429: "Too many attempts. Please wait and try again.",
  503: "The service is temporarily unavailable. Please try again.",
};

export function isNetworkFailure(error: unknown) {
  if (error instanceof TypeError) return true;
  const message = error instanceof Error ? error.message : String(error ?? "");
  return /failed to fetch|networkerror|load failed|network request failed/i.test(message);
}

export function userFacingHttpError(status: number, fallback: string) {
  if (STATUS_MESSAGES[status]) return STATUS_MESSAGES[status];
  if (status >= 500) return fallback;
  return fallback;
}

export function userFacingApiMessage(
  status: number,
  payloadMessage: string | undefined,
  fallback: string,
) {
  if (STATUS_MESSAGES[status]) return STATUS_MESSAGES[status];
  if (status >= 500) return fallback;
  const payload = payloadMessage?.trim();
  return payload || fallback;
}

export function userFacingCaughtError(error: unknown, fallback: string) {
  if (isNetworkFailure(error)) {
    return "Network error. Check your connection and try again.";
  }
  if (error && typeof error === "object" && "status" in error && typeof error.status === "number") {
    const message = error instanceof Error ? error.message : fallback;
    return userFacingHttpError(error.status, message || fallback);
  }
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}
