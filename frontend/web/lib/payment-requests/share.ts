type EnvSource = Record<string, string | undefined>;

export function appOrigin(
  location: Pick<Location, "origin"> | undefined = typeof window !== "undefined"
    ? window.location
    : undefined,
) {
  return location?.origin?.replace(/\/$/, "") ?? "";
}

export function paymentRequestPath(publicId: string) {
  return "/pay/" + encodeURIComponent(publicId);
}

export function configuredPublicOrigin(source: EnvSource = process.env) {
  return source.NEXT_PUBLIC_NIMNEAR_PUBLIC_ORIGIN?.trim().replace(/\/$/, "") ?? "";
}

export function isLocalDevelopmentOrigin(origin: string) {
  try {
    const parsed = new URL(origin);
    return (
      parsed.protocol === "http:" &&
      (parsed.hostname === "localhost" || parsed.hostname === "127.0.0.1" || parsed.hostname === "[::1]")
    );
  } catch {
    return false;
  }
}

export function isSecurePaymentShareOrigin(
  origin: string,
  nodeEnv = process.env.NODE_ENV,
) {
  if (!origin) return false;
  try {
    const parsed = new URL(origin);
    if (parsed.protocol === "https:") {
      return nodeEnv !== "production" || !parsed.hostname.endsWith(".vercel.app");
    }
    return nodeEnv !== "production" && isLocalDevelopmentOrigin(origin);
  } catch {
    return false;
  }
}

export function paymentShareOrigin(
  location: Pick<Location, "origin"> | undefined = typeof window !== "undefined"
    ? window.location
    : undefined,
  source: EnvSource = process.env,
) {
  const configured = configuredPublicOrigin(source);
  if (configured) return configured;
  if (source.NODE_ENV === "production") return "";
  return appOrigin(location);
}

export function paymentRequestShareUrl(
  publicId: string,
  origin = paymentShareOrigin(),
  nodeEnv = process.env.NODE_ENV,
) {
  const path = paymentRequestPath(publicId);
  if (!origin || !isSecurePaymentShareOrigin(origin, nodeEnv)) return "";
  return origin + path;
}

export function paymentRequestShareText(input: {
  amountNim: string;
  note?: string | null;
}) {
  const amount = `${input.amountNim} NIM`;
  const note = input.note?.trim();
  if (note) return `NIMNear payment request: ${amount} — ${note}`;
  return `NIMNear payment request: ${amount}`;
}

export function shareUrlContainsSecrets(url: string) {
  try {
    const parsed = new URL(url, "https://nimnear.invalid");
    const haystack = `${parsed.search} ${parsed.hash} ${parsed.pathname}`.toLowerCase();
    return (
      parsed.searchParams.has("token") ||
      parsed.searchParams.has("jwt") ||
      parsed.searchParams.has("access_token") ||
      haystack.includes("bearer ") ||
      haystack.includes("jwt=")
    );
  } catch {
    return /token=|jwt=|access_token=/i.test(url);
  }
}

export function canUseWebShare(
  share: Navigator["share"] | undefined = typeof navigator !== "undefined"
    ? navigator.share?.bind(navigator)
    : undefined,
) {
  return typeof share === "function";
}

export async function copyText(
  value: string,
  clipboard: Pick<Clipboard, "writeText"> | undefined = typeof navigator !== "undefined"
    ? navigator.clipboard
    : undefined,
) {
  if (!clipboard?.writeText) {
    throw new Error("Clipboard is not available.");
  }
  await clipboard.writeText(value);
}

export async function sharePaymentRequest(
  input: { title: string; text: string; url: string },
  share: Navigator["share"] | undefined = typeof navigator !== "undefined"
    ? navigator.share?.bind(navigator)
    : undefined,
) {
  if (!canUseWebShare(share) || !share) return false;
  await share({
    title: input.title,
    text: input.text,
    url: input.url,
  });
  return true;
}
