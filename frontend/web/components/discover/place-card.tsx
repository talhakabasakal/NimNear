import { MapPin } from "lucide-react";
import Link from "next/link";

import { ExternalMediaImage } from "@/components/media/external-media-image";
import type { NearbyPlaceRecord } from "@/lib/api/places";

function formatDistance(distanceMeters: number) {
  if (distanceMeters < 1000) return Math.max(1, Math.round(distanceMeters)) + " m away";
  return (distanceMeters / 1000).toFixed(1).replace(".0", "") + " km away";
}

export function PlaceCard({ place }: { place: NearbyPlaceRecord }) {
  const fallback = <div className="grid size-full place-items-center bg-[radial-gradient(circle_at_25%_20%,#8277ff_0,transparent_32%),linear-gradient(135deg,#251e3a,#171621_60%,#382447)]" aria-hidden="true"><MapPin size={22} className="text-accent/80" /></div>;

  return (
    <Link href={`/places/${place.id}`} className="group block rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={place.name}>
      <article className="overflow-hidden rounded-xl border border-border bg-surface transition-colors group-hover:bg-surface-hover">
        <div className="relative h-28 overflow-hidden bg-surface-hover">
          <ExternalMediaImage src={place.image_url || null} alt="" className="size-full object-cover" fallback={fallback} />
          <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-[#0f0e10]/70 via-transparent to-transparent" />
        </div>
        <div className="space-y-2 p-4">
          <div>
            <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-accent">{place.category || "Discovery spot"}</p>
            <h3 className="mt-1 truncate text-[15px] font-semibold text-foreground">{place.name}</h3>
          </div>
          <p className="flex items-center gap-2 text-xs text-muted"><MapPin size={14} className="text-accent/80" />{formatDistance(place.distance_meters)}</p>
          {place.address ? <p className="truncate text-xs text-muted">{place.address}</p> : null}
        </div>
      </article>
    </Link>
  );
}
