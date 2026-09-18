import { apiBaseUrl } from "../api/events";

export function websocketUrl(baseUrl = apiBaseUrl): string {
  const url = new URL("/api/v1/ws", baseUrl);
  url.search = "";
  if (url.protocol === "https:") url.protocol = "wss:";
  else url.protocol = "ws:";
  return url.toString();
}

export function websocketUrlHasSecret(url: string): boolean {
  try {
    const parsed = new URL(url);
    const haystack = `${parsed.search} ${parsed.hash}`.toLowerCase();
    return (
      parsed.searchParams.has("token") ||
      haystack.includes("jwt") ||
      haystack.includes("bearer")
    );
  } catch {
    return true;
  }
}
