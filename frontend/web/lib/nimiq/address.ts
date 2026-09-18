const NIMIQ_ALPHABET = "0123456789ABCDEFGHJKLMNPQRSTUVXY";

export function compactNimiqAddress(address: string) {
  return address.replace(/\s/g, "").toUpperCase();
}

export function formatNimiqAddress(compact: string) {
  const normalized = compactNimiqAddress(compact);
  const parts: string[] = [];
  for (let index = 0; index < normalized.length; index += 4) {
    parts.push(normalized.slice(index, index + 4));
  }
  return parts.join(" ");
}

function ibanMod97(value: string) {
  let remainder = 0;
  for (const char of value) {
    if (char >= "0" && char <= "9") {
      remainder = (remainder * 10 + (char.charCodeAt(0) - 48)) % 97;
    } else if (char >= "A" && char <= "Z") {
      remainder = (remainder * 100 + (char.charCodeAt(0) - 55)) % 97;
    } else {
      return -1;
    }
  }
  return remainder;
}

/** Validates IBAN checksum and returns the canonical spaced NQ address. */
export function normalizeNimiqAddress(value: string): string | null {
  const compact = compactNimiqAddress(value.trim());
  if (compact.length !== 36 || !compact.startsWith("NQ")) return null;
  for (const char of compact.slice(4)) {
    if (!NIMIQ_ALPHABET.includes(char)) return null;
  }
  if (ibanMod97(compact.slice(4) + compact.slice(0, 4)) !== 1) return null;
  return formatNimiqAddress(compact);
}

export function isSameNimiqAddress(left: string, right: string) {
  const normalizedLeft = normalizeNimiqAddress(left);
  const normalizedRight = normalizeNimiqAddress(right);
  if (normalizedLeft && normalizedRight) {
    return compactNimiqAddress(normalizedLeft) === compactNimiqAddress(normalizedRight);
  }
  const compactLeft = compactNimiqAddress(left);
  const compactRight = compactNimiqAddress(right);
  return compactLeft.length > 0 && compactLeft === compactRight;
}

export function shortenNimiqAddress(address: string) {
  const compact = compactNimiqAddress(address);
  if (compact.length <= 8) return compact;
  return compact.slice(0, 4) + "..." + compact.slice(-2);
}

export function identiconSeeds(address: string, count: number) {
  const compact = compactNimiqAddress(address);
  return Array.from({ length: count }, (_, index) =>
    index === 0 ? compact : compact + ":face:" + index,
  );
}
