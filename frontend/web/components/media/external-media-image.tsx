"use client";

import { useState, type ReactNode } from "react";

type ExternalMediaImageProps = {
  src: string | null;
  alt?: string;
  className: string;
  fallback: ReactNode;
};

function validMediaUrl(value: string | null): value is string {
  if (value === null || value.length === 0 || value.length > 2048 || /\s/.test(value)) return false;
  try {
    const parsed = new URL(value);
    return (parsed.protocol === "http:" || parsed.protocol === "https:") && parsed.host.length > 0 && parsed.username === "" && parsed.password === "";
  } catch {
    return false;
  }
}

// Stored legacy URLs also pass this guard before reaching an image element.
export function ExternalMediaImage({ src, alt = "", className, fallback }: ExternalMediaImageProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  if (!validMediaUrl(src) || failedSrc === src) return <>{fallback}</>;
  return <img src={src} alt={alt} className={className} loading="lazy" decoding="async" referrerPolicy="no-referrer" onError={() => setFailedSrc(src)} />;
}
