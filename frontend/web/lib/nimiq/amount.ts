export const LUNAS_PER_NIM = BigInt("100000");
export const MAX_INT64 = BigInt("9223372036854775807");
export const MAX_SAFE_LUNA = BigInt(Number.MAX_SAFE_INTEGER);

/** Converts a decimal NIM string to exact Luna without using floating point. */
export function nimToLunas(value: string): bigint | null {
  const normalized = value.trim();
  if (!normalized || !/^[0-9]+(?:\.[0-9]+)?$/.test(normalized)) return null;

  const [integerPart, fractionPart = ""] = normalized.split(".");
  if (fractionPart.length > 5) return null;

  const fraction = BigInt((fractionPart + "00000").slice(0, 5));
  const lunas = BigInt(integerPart) * LUNAS_PER_NIM + fraction;
  return lunas <= MAX_INT64 ? lunas : null;
}

/** Canonical exact decimal string for a Luna integer. */
export function lunasToNimString(lunas: bigint): string {
  if (lunas === BigInt("0")) return "0";
  const integer = lunas / LUNAS_PER_NIM;
  const fraction = (lunas % LUNAS_PER_NIM)
    .toString()
    .padStart(5, "0")
    .replace(/0+$/, "");
  return fraction ? `${integer}.${fraction}` : integer.toString();
}

/** Returns the canonical exact decimal string accepted by POST /events. */
export function normalizeNimPrice(value: string): string | null {
  const lunas = nimToLunas(value);
  if (lunas === null) return null;
  return lunasToNimString(lunas);
}

export function formatNimPrice(price: string) {
  return normalizeNimPrice(price) ?? price.trim();
}

export function parseBalanceLunas(value: string): bigint | null {
  const normalized = value.trim();
  if (!/^[0-9]+$/.test(normalized)) return null;
  try {
    return BigInt(normalized);
  } catch {
    return null;
  }
}
