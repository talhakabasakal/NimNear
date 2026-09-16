"use client";

import { MapPin } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { StateCard } from "@/components/app/state-card";
import { Button } from "@/components/ui/button";
import { fetchNearbyPlaces, type NearbyPlaceRecord } from "@/lib/api/places";
import { locationStateForError, nearbyResultState, type LocationState } from "@/lib/location-state";

import { PlaceCard } from "./place-card";

function ContinueBrowsing() {
  return <Link href="/events" className="text-xs font-medium text-accent transition-colors hover:text-foreground">Konum paylaşmadan etkinliklere göz at</Link>;
}

function locationDescription(state: LocationState) {
  if (state === "denied") return "Konum izni verilmedi. Yakındaki yerleri görmek için izni açabilir veya konum paylaşmadan devam edebilirsin.";
  if (state === "unavailable") return "Bu tarayıcı veya WebView konum bilgisini kullanamıyor.";
  if (state === "timeout") return "Konum bilgisi zamanında alınamadı. Tekrar deneyebilirsin.";
  return "Yakındaki yerler şu anda alınamıyor. Tekrar deneyebilirsin.";
}

export function NearbyPlaceDiscovery() {
  const [state, setState] = useState<LocationState>("idle");
  const [places, setPlaces] = useState<NearbyPlaceRecord[]>([]);

  function useLocation() {
    if (typeof navigator === "undefined" || !navigator.geolocation) {
      setState("unavailable");
      return;
    }

    setState("locating");
    navigator.geolocation.getCurrentPosition(
      (position) => {
        setState("loading");
        void fetchNearbyPlaces({ latitude: position.coords.latitude, longitude: position.coords.longitude })
          .then((nextPlaces) => {
            setPlaces(nextPlaces);
            setState(nearbyResultState(nextPlaces));
          })
          .catch(() => setState("error"));
      },
      (error) => setState(locationStateForError(error.code)),
      { enableHighAccuracy: false, maximumAge: 300000, timeout: 10000 },
    );
  }

  return (
    <section>
      <div className="mb-4 flex items-end justify-between gap-4">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Yakındaki yerler</p>
          <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">Konumuna göre keşfet</h2>
        </div>
        {state === "success" ? <span className="rounded-full border border-border-faint px-3 py-1 text-[11px] font-medium text-muted">Gerçek sonuçlar</span> : null}
      </div>

      {state === "idle" ? (
        <div className="rounded-xl border border-border bg-surface p-5 sm:p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div><p className="text-sm font-medium text-foreground">Çevrendeki keşif noktalarını bul</p><p className="mt-1 text-xs leading-5 text-muted">Konumun yalnızca yakındaki yerleri aramak için kullanılır ve profilinde saklanmaz.</p></div>
            <Button type="button" onClick={useLocation}><MapPin size={15} /> Konumumu kullan</Button>
          </div>
          <div className="mt-4"><ContinueBrowsing /></div>
        </div>
      ) : null}

      {state === "locating" || state === "loading" ? <div className="h-40 animate-pulse rounded-xl border border-border bg-surface" aria-label="Konum ve yakın yerler yükleniyor" /> : null}

      {state === "denied" || state === "unavailable" || state === "timeout" || state === "error" ? (
        <div className="space-y-4">
          <StateCard kind="error" title={state === "denied" ? "Konum izni gerekli" : "Konum kullanılamıyor"} description={locationDescription(state)} />
          <div className="flex flex-wrap items-center justify-center gap-4"><Button type="button" variant="outline" onClick={useLocation}>Tekrar dene</Button><ContinueBrowsing /></div>
        </div>
      ) : null}

      {state === "empty" ? (
        <div className="space-y-4">
          <StateCard kind="empty" title="Yakınında yer bulunamadı" description="Bu konum ve arama yarıçapında aktif bir keşif noktası yok." />
          <div className="flex flex-wrap items-center justify-center gap-4"><Button type="button" variant="outline" onClick={useLocation}>Tekrar ara</Button><ContinueBrowsing /></div>
        </div>
      ) : null}

      {state === "success" ? <div className="grid gap-3 sm:grid-cols-2">{places.map((place) => <PlaceCard key={place.id} place={place} />)}</div> : null}
    </section>
  );
}
