"use client";

import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

const identiconCache = new Map<string, string>();
const placeholder =
  "data:image/svg+xml;utf8," +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160"><path fill="#bbb" d="M80 8l64 36v72L80 152 16 116V44z"/></svg>',
  );

declare global {
  interface Window {
    NIMIQ_IDENTICONS_SVG_PATH?: string;
  }
}

export function NimiqIdenticon({
  seed,
  alt = "",
  className,
}: {
  seed: string;
  alt?: string;
  className?: string;
}) {
  const cached = identiconCache.get(seed);
  const [, setVersion] = useState(0);

  useEffect(() => {
    if (identiconCache.has(seed)) return;

    window.NIMIQ_IDENTICONS_SVG_PATH = "/identicons.min.svg";
    let cancelled = false;
    void import("@nimiq/identicons/dist/identicons.bundle.min.js")
      .then((module) => module.default.toDataUrl(seed))
      .then((url) => {
        identiconCache.set(seed, url);
        if (!cancelled) setVersion((current) => current + 1);
      })
      .catch(() => {
        identiconCache.set(seed, placeholder);
        if (!cancelled) setVersion((current) => current + 1);
      });

    return () => {
      cancelled = true;
    };
  }, [seed]);

  return (
    <img
      src={cached || placeholder}
      alt={alt}
      className={cn("block aspect-square bg-transparent", className)}
      width={160}
      height={160}
    />
  );
}
