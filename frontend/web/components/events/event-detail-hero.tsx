"use client";

import { ImageOff } from "lucide-react";
import { useState } from "react";

type EventDetailHeroProps = {
  imageUrl: string;
  title: string;
};

export function EventDetailHero({ imageUrl, title }: EventDetailHeroProps) {
  const [hasImageError, setHasImageError] = useState(false);

  if (!imageUrl || hasImageError) {
    return (
      <div className="relative grid min-h-56 place-items-center overflow-hidden rounded-xl border border-border bg-[radial-gradient(circle_at_25%_20%,#8277ff_0,transparent_32%),linear-gradient(135deg,#251e3a,#171621_60%,#382447)] sm:min-h-72" aria-label={`${title} görseli mevcut değil`} role="img">
        <div className="absolute -right-10 -top-12 size-44 rounded-full border border-accent/20" />
        <div className="absolute bottom-0 left-0 h-1/2 w-full bg-gradient-to-t from-[#0f0e10]/60 to-transparent" />
        <ImageOff size={22} className="relative text-accent/70" aria-hidden="true" />
      </div>
    );
  }

  return (
    <div className="relative min-h-56 overflow-hidden rounded-xl border border-border bg-surface sm:min-h-72">
      <img src={imageUrl} alt="" className="size-full min-h-56 object-cover sm:min-h-72" onError={() => setHasImageError(true)} />
      <div className="absolute inset-0 bg-gradient-to-t from-[#0f0e10]/75 via-transparent to-transparent" />
    </div>
  );
}
