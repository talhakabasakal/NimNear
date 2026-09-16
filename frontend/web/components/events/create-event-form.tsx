"use client";

import { CalendarDays, Clock3, Image, MapPin, Ticket, Users } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { Button } from "@/components/ui/button";
import {
  AuthApiError,
  clearAuthSession,
  fetchCurrentUser,
  readAuthSession,
  type AuthSession,
  writeAuthSession,
} from "@/lib/api/auth";
import { CalendarsApiError, fetchMyCalendars, type CalendarRecord } from "@/lib/api/calendars";
import { createEvent, EventsApiError, normalizeNimPrice, type CreateEventInput } from "@/lib/api/events";
import { fetchNearbyPlaces, type NearbyPlaceRecord } from "@/lib/api/places";

type FormValues = {
  title: string;
  description: string;
  startsAt: string;
  endsAt: string;
  city: string;
  address: string;
  priceNim: string;
  capacity: string;
  imageURL: string;
  latitude: string;
  longitude: string;
};

type FormErrors = Partial<Record<keyof FormValues, string>> & { form?: string };
type AuthStatus = "checking" | "anonymous" | "authenticated" | "error";
type CalendarStatus = "loading" | "ready" | "error";
type PlaceStatus = "idle" | "loading" | "ready" | "empty" | "error";

const initialValues: FormValues = {
  title: "",
  description: "",
  startsAt: "",
  endsAt: "",
  city: "",
  address: "",
  priceNim: "",
  capacity: "",
  imageURL: "",
  latitude: "",
  longitude: "",
};

const inputClass = "mt-2 h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30";
const labelClass = "text-xs font-medium text-foreground";

function FieldError({ message }: { message?: string }) {
  return message ? <p className="mt-1 text-xs text-red-200">{message}</p> : null;
}

function isValidMediaURL(value: string) {
  if (!value) return true;
  if (value.length > 2048 || value.trim() !== value || Array.from(value).some((character) => { const code = character.charCodeAt(0); return /\s/.test(character) || code < 32 || code === 127; })) return false;
  try {
    const parsed = new URL(value);
    return (parsed.protocol === "http:" || parsed.protocol === "https:") && !parsed.username && !parsed.password;
  } catch {
    return false;
  }
}

function serializeDateTimeLocal(value: string) {
  const parsed = value ? new Date(value) : null;
  return parsed && !Number.isNaN(parsed.getTime()) ? parsed.toISOString() : null;
}

function validateForm(values: FormValues, isFree: boolean): FormErrors {
  const errors: FormErrors = {};
  if (!values.title.trim()) errors.title = "Etkinlik adı gerekli.";
  if (values.title.trim().length > 255) errors.title = "Etkinlik adı 255 karakteri geçemez.";
  if (!values.startsAt) errors.startsAt = "Başlangıç zamanı gerekli.";
  if (!values.endsAt) errors.endsAt = "Bitiş zamanı gerekli.";

  const startsAt = serializeDateTimeLocal(values.startsAt);
  const endsAt = serializeDateTimeLocal(values.endsAt);
  if (values.startsAt && !startsAt) errors.startsAt = "Geçerli bir başlangıç zamanı seç.";
  if (values.endsAt && !endsAt) errors.endsAt = "Geçerli bir bitiş zamanı seç.";
  if (startsAt && endsAt && new Date(endsAt) <= new Date(startsAt)) errors.endsAt = "Bitiş zamanı başlangıçtan sonra olmalı.";

  if (!values.city.trim()) errors.city = "Şehir gerekli.";
  if (values.city.trim().length > 120) errors.city = "Şehir 120 karakteri geçemez.";
  if (values.address.trim().length > 500) errors.address = "Adres 500 karakteri geçemez.";
  if (values.description.length > 5000) errors.description = "Açıklama 5000 karakteri geçemez.";
  if (!isFree) {
    const normalizedPrice = normalizeNimPrice(values.priceNim);
    if (!values.priceNim.trim()) errors.priceNim = "Ücretli etkinlik için NIM fiyatı gerekli.";
    else if (!normalizedPrice) errors.priceNim = "Fiyat en fazla 5 ondalık basamak içeren, yuvarlama gerektirmeyen bir NIM tutarı olmalı.";
    else if (normalizedPrice === "0") errors.priceNim = "Ücretsiz etkinlik için Ücretsiz seçeneğini kullan.";
  }
  if (!values.capacity.trim()) {
    // Empty capacity is the explicit unlimited-capacity state.
  } else if (!/^[0-9]+$/.test(values.capacity.trim()) || !Number.isSafeInteger(Number(values.capacity)) || Number(values.capacity) < 1 || Number(values.capacity) > 2147483647) {
    errors.capacity = "Kapasite 1 ile 2.147.483.647 arasında bir tam sayı olmalı.";
  }
  if (!isValidMediaURL(values.imageURL.trim())) errors.imageURL = "Görsel mutlak bir HTTP(S) URL'si olmalı ve 2048 karakteri geçmemeli.";

  const hasLatitude = Boolean(values.latitude.trim());
  const hasLongitude = Boolean(values.longitude.trim());
  if (hasLatitude !== hasLongitude) {
    errors.latitude = "Enlem ve boylam birlikte girilmeli.";
    errors.longitude = "Enlem ve boylam birlikte girilmeli.";
  } else if (hasLatitude && hasLongitude) {
    const latitude = Number(values.latitude);
    const longitude = Number(values.longitude);
    if (!Number.isFinite(latitude) || latitude < -90 || latitude > 90) errors.latitude = "Enlem -90 ile 90 arasında olmalı.";
    if (!Number.isFinite(longitude) || longitude < -180 || longitude > 180) errors.longitude = "Boylam -180 ile 180 arasında olmalı.";
  }
  return errors;
}

function browserLocationError(code: number) {
  if (code === 1) return "Konum izni verilmedi.";
  if (code === 2) return "Konum alınamadı.";
  if (code === 3) return "Konum isteği zaman aşımına uğradı.";
  return "Konum alınamadı.";
}

export function CreateEventForm() {
  const router = useRouter();
  const [authStatus, setAuthStatus] = useState<AuthStatus>("checking");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [values, setValues] = useState<FormValues>(initialValues);
  const [isFree, setIsFree] = useState(true);
  const [selectedCalendarId, setSelectedCalendarId] = useState("");
  const [ownedCalendars, setOwnedCalendars] = useState<CalendarRecord[]>([]);
  const [calendarStatus, setCalendarStatus] = useState<CalendarStatus>("loading");
  const [calendarError, setCalendarError] = useState<string | null>(null);
  const [calendarRetryKey, setCalendarRetryKey] = useState(0);
  const [places, setPlaces] = useState<NearbyPlaceRecord[]>([]);
  const [selectedPlaceId, setSelectedPlaceId] = useState("");
  const [placeStatus, setPlaceStatus] = useState<PlaceStatus>("idle");
  const [placeError, setPlaceError] = useState<string | null>(null);
  const [errors, setErrors] = useState<FormErrors>({});
  const [pending, setPending] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const stored = readAuthSession();
    if (!stored) {
      setAuthStatus("anonymous");
      return () => { cancelled = true; };
    }

    fetchCurrentUser(stored.token).then((user) => {
      if (cancelled) return;
      const refreshed = { ...stored, user };
      writeAuthSession(refreshed);
      setSession(refreshed);
      setAuthStatus("authenticated");
    }).catch((error: unknown) => {
      if (cancelled) return;
      if (error instanceof AuthApiError && error.status === 401) {
        clearAuthSession();
        setAuthStatus("anonymous");
        return;
      }
      setAuthError(error instanceof Error ? error.message : "Oturum durumu alınamadı.");
      setAuthStatus("error");
    });
    return () => { cancelled = true; };
  }, [retryKey]);

  useEffect(() => {
    if (authStatus !== "authenticated" || !session) return;
    let cancelled = false;
    setCalendarStatus("loading");
    setCalendarError(null);
    fetchMyCalendars(session.token).then((result) => {
      if (cancelled) return;
      setOwnedCalendars(result.owned);
      setCalendarStatus("ready");
    }).catch((error: unknown) => {
      if (cancelled) return;
      if (error instanceof CalendarsApiError && error.status === 401) {
        clearAuthSession();
        setSession(null);
        setAuthStatus("anonymous");
        return;
      }
      setCalendarError(error instanceof Error ? error.message : "Takvimler yüklenemedi.");
      setCalendarStatus("error");
    });
    return () => { cancelled = true; };
  }, [authStatus, calendarRetryKey, session]);

  function updateValue(key: keyof FormValues, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: undefined, form: undefined }));
  }

  function selectPlace(placeID: string) {
    setSelectedPlaceId(placeID);
    setErrors((current) => ({ ...current, address: undefined, latitude: undefined, longitude: undefined, form: undefined }));
    if (placeID) setValues((current) => ({ ...current, address: "", latitude: "", longitude: "" }));
  }

  function loadNearbyPlaceOptions() {
    if (!navigator.geolocation) {
      setPlaceError("Bu ortam konum erişimini desteklemiyor.");
      setPlaceStatus("error");
      return;
    }
    setPlaceStatus("loading");
    setPlaceError(null);
    navigator.geolocation.getCurrentPosition((position) => {
      void fetchNearbyPlaces({ latitude: position.coords.latitude, longitude: position.coords.longitude })
        .then((nearby) => {
          setPlaces(nearby);
          setSelectedPlaceId("");
          setPlaceStatus(nearby.length > 0 ? "ready" : "empty");
        })
        .catch((error: unknown) => {
          setPlaceError(error instanceof Error ? error.message : "Yakındaki yerler yüklenemedi.");
          setPlaceStatus("error");
        });
    }, (error) => {
      setPlaceError(browserLocationError(error.code));
      setPlaceStatus("error");
    }, { enableHighAccuracy: false, timeout: 10000, maximumAge: 300000 });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!session || pending) return;

    const nextErrors = validateForm(values, isFree);
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      return;
    }

    const startsAt = serializeDateTimeLocal(values.startsAt);
    const endsAt = serializeDateTimeLocal(values.endsAt);
    const priceNim = isFree ? "0" : normalizeNimPrice(values.priceNim);
    if (!startsAt || !endsAt || !priceNim) {
      setErrors({ form: "Etkinlik bilgileri geçerli değil. Alanları kontrol et." });
      return;
    }

    const input: CreateEventInput = {
      title: values.title.trim(),
      description: values.description.trim(),
      starts_at: startsAt,
      ends_at: endsAt,
      price_nim: priceNim,
      currency: "NIM",
      city: values.city.trim(),
    };
    if (values.imageURL.trim()) input.image_url = values.imageURL.trim();
    if (selectedCalendarId) input.calendar_id = selectedCalendarId;
    if (selectedPlaceId) {
      input.place_id = selectedPlaceId;
    } else {
      if (values.address.trim()) input.address = values.address.trim();
      if (values.latitude.trim() && values.longitude.trim()) {
        input.latitude = Number(values.latitude);
        input.longitude = Number(values.longitude);
      }
    }
    if (values.capacity.trim()) input.capacity = Number(values.capacity);

    setPending(true);
    setErrors({});
    try {
      const created = await createEvent(input, session.token);
      router.push(`/events/${created.id}`);
    } catch (error) {
      if (error instanceof EventsApiError && error.status === 401) {
        clearAuthSession();
        setSession(null);
        setAuthStatus("anonymous");
        setErrors({ form: "Oturumun sona ermiş. Nimiq imzası ile backend oturumu yenileme desteği bekleniyor." });
      } else {
        setErrors({ form: error instanceof EventsApiError ? error.message : "Etkinlik oluşturulamadı. Tekrar deneyebilirsin." });
      }
    } finally {
      setPending(false);
    }
  }

  if (authStatus === "checking") return <div className="h-48 animate-pulse rounded-xl border border-border bg-surface" aria-label="Kimlik doğrulama kontrol ediliyor" />;
  if (authStatus === "error") return <section className="rounded-xl border border-red-300/20 bg-red-400/10 p-5"><p className="text-sm font-medium text-foreground">Oturum durumu yüklenemedi.</p><p className="mt-1 text-xs leading-5 text-muted">{authError ?? "Kimlik servisine şu anda ulaşılamıyor."}</p><Button type="button" variant="outline" className="mt-4" onClick={() => { setAuthError(null); setAuthStatus("checking"); setRetryKey((value) => value + 1); }}>Tekrar dene</Button></section>;
  if (authStatus === "anonymous") return <NimiqConnect description="Nimiq Pay hesabını bağlayabilirsin. Bu bağlantı henüz NIMNear etkinlik oluşturma oturumu oluşturmaz." blockedMessage="Etkinlik oluşturmak için Nimiq imzası ile backend oturumu oluşturma desteği bekleniyor." />;

  const selectedPlace = places.find((place) => place.id === selectedPlaceId) ?? null;

  return (
    <form className="space-y-6" onSubmit={handleSubmit}>
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-surface px-4 py-3"><div><p className="text-xs text-muted">Oturum açıldı</p><p className="text-sm font-medium text-foreground">{session?.user.first_name} {session?.user.last_name} <span className="font-normal text-muted">· {session?.user.email}</span></p></div><button type="button" className="text-xs font-medium text-muted transition-colors hover:text-foreground" onClick={() => { clearAuthSession(); setSession(null); setAuthStatus("anonymous"); }}>Çıkış yap</button></div>

      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="mb-5"><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Temel bilgiler</p><h2 className="mt-1 text-xl font-semibold text-foreground">Etkinliğini tanımla</h2></div>
        <div className="space-y-5">
          <label className={labelClass}>Etkinlik adı<input className={inputClass} value={values.title} onChange={(event) => updateValue("title", event.target.value)} placeholder="Örn. Akşam buluşması" maxLength={255} /> <FieldError message={errors.title} /></label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className={labelClass}><span className="inline-flex items-center gap-2"><CalendarDays size={14} className="text-accent" />Başlangıç</span><input type="datetime-local" className={inputClass} value={values.startsAt} onChange={(event) => updateValue("startsAt", event.target.value)} /> <FieldError message={errors.startsAt} /></label>
            <label className={labelClass}><span className="inline-flex items-center gap-2"><Clock3 size={14} className="text-accent" />Bitiş</span><input type="datetime-local" className={inputClass} value={values.endsAt} onChange={(event) => updateValue("endsAt", event.target.value)} /> <FieldError message={errors.endsAt} /></label>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className={labelClass}><span className="inline-flex items-center gap-2"><MapPin size={14} className="text-accent" />Şehir</span><input className={inputClass} value={values.city} onChange={(event) => updateValue("city", event.target.value)} maxLength={120} /> <FieldError message={errors.city} /></label>
            <label className={labelClass}><span className="inline-flex items-center gap-2"><Image size={14} className="text-accent" />Görsel URL'si</span><input type="url" className={inputClass} value={values.imageURL} onChange={(event) => updateValue("imageURL", event.target.value)} placeholder="https://…" maxLength={2048} /> <FieldError message={errors.imageURL} /></label>
          </div>
          <label className={labelClass}>Açıklama<textarea className="mt-2 min-h-32 w-full resize-y rounded-lg border border-border bg-background px-3 py-3 text-sm leading-6 text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30" value={values.description} onChange={(event) => updateValue("description", event.target.value)} placeholder="Etkinlik hakkında kısa bilgi" maxLength={5000} /> <FieldError message={errors.description} /></label>
        </div>
      </section>

      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="mb-5"><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Takvim ve konum</p><h2 className="mt-1 text-xl font-semibold text-foreground">Etkinliğini sınıflandır</h2></div>
        <div className="space-y-5">
          <label className={labelClass}>Takvim<select className={inputClass} value={selectedCalendarId} onChange={(event) => setSelectedCalendarId(event.target.value)} disabled={calendarStatus === "loading"}><option value="">Takvim seçmeden devam et</option>{ownedCalendars.map((calendar) => <option key={calendar.id} value={calendar.id}>{calendar.name}</option>)}</select></label>
          {calendarStatus === "loading" ? <p className="text-xs text-muted" role="status">Sahip olduğun takvimler yükleniyor…</p> : null}
          {calendarStatus === "error" ? <div className="flex flex-wrap items-center gap-3 text-xs text-red-200"><span>{calendarError ?? "Takvimler yüklenemedi."} Takvim seçmeden devam edebilirsin.</span><Button type="button" variant="outline" size="sm" onClick={() => setCalendarRetryKey((value) => value + 1)}>Tekrar dene</Button></div> : null}
          {calendarStatus === "ready" && ownedCalendars.length === 0 ? <p className="text-xs text-muted">Henüz sahip olduğun bir takvim yok. Takvim seçmeden devam edebilirsin.</p> : null}

          <div className="rounded-lg border border-border bg-background/50 p-4">
            <div className="flex flex-wrap items-center justify-between gap-3"><div><p className="text-xs font-medium text-foreground">Kayıtlı yer</p><p className="mt-1 text-xs leading-5 text-muted">Sadece gerçek backend yerleri arasından seçim yapabilirsin.</p></div><Button type="button" variant="outline" size="sm" onClick={loadNearbyPlaceOptions} disabled={placeStatus === "loading"}>{placeStatus === "loading" ? "Yerler yükleniyor…" : "Yakındaki yerleri yükle"}</Button></div>
            {placeStatus === "error" ? <p className="mt-3 text-xs text-red-200" role="alert">{placeError}</p> : null}
            {placeStatus === "empty" ? <p className="mt-3 text-xs text-muted">Bu konum çevresinde seçilebilir kayıtlı yer yok. Özel adres ekleyebilirsin.</p> : null}
            {places.length > 0 ? <label className={labelClass + " mt-4 block"}>Yer seç<select className={inputClass} value={selectedPlaceId} onChange={(event) => selectPlace(event.target.value)}><option value="">Özel adres kullan</option>{places.map((place) => <option key={place.id} value={place.id}>{place.name} · {Math.round(place.distance_meters)} m</option>)}</select></label> : null}
            {selectedPlace ? <p className="mt-3 text-xs leading-5 text-muted">{selectedPlace.name} seçildi · {selectedPlace.address}. Bu kayıtlı yerin kimliği gönderilir; özel adres ve koordinat alanları kullanılmaz.</p> : null}
          </div>

          {!selectedPlace ? <>
            <label className={labelClass}>Özel adres<input className={inputClass} value={values.address} onChange={(event) => updateValue("address", event.target.value)} placeholder="Buluşma noktası veya adres" maxLength={500} /> <FieldError message={errors.address} /></label>
            <div><p className="text-xs font-medium text-foreground">Özel koordinatlar (isteğe bağlı)</p><div className="grid gap-4 sm:grid-cols-2"><label className={labelClass}>Enlem<input inputMode="decimal" className={inputClass} value={values.latitude} onChange={(event) => updateValue("latitude", event.target.value)} placeholder="Enlem" /> <FieldError message={errors.latitude} /></label><label className={labelClass}>Boylam<input inputMode="decimal" className={inputClass} value={values.longitude} onChange={(event) => updateValue("longitude", event.target.value)} placeholder="Boylam" /> <FieldError message={errors.longitude} /></label></div></div>
          </> : null}
        </div>
      </section>

      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="mb-5"><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Katılım</p><h2 className="mt-1 text-xl font-semibold text-foreground">Bilet ve kapasite</h2></div>
        <div className="grid gap-4 sm:grid-cols-2">
          <div><div className="flex items-center justify-between gap-3"><label className={labelClass} htmlFor="priceNim"><span className="inline-flex items-center gap-2"><Ticket size={14} className="text-accent" />NIM fiyatı</span></label><label className="inline-flex items-center gap-2 text-xs text-muted"><input type="checkbox" checked={isFree} onChange={(event) => { setIsFree(event.target.checked); setErrors((current) => ({ ...current, priceNim: undefined })); }} /> Ücretsiz</label></div><input id="priceNim" inputMode="decimal" className={`${inputClass} disabled:cursor-not-allowed disabled:opacity-50`} value={values.priceNim} onChange={(event) => updateValue("priceNim", event.target.value)} placeholder="0.00000" disabled={isFree} /> <FieldError message={errors.priceNim} /><p className="mt-1 text-xs text-muted">1 NIM = 100.000 Luna. En fazla 5 ondalık basamak; yuvarlama yapılmaz.</p></div>
          <label className={labelClass}><span className="inline-flex items-center gap-2"><Users size={14} className="text-accent" />Kapasite</span><input inputMode="numeric" className={inputClass} value={values.capacity} onChange={(event) => updateValue("capacity", event.target.value)} placeholder="Sınırsız" /> <FieldError message={errors.capacity} /><p className="mt-1 text-xs text-muted">Boş bırakılırsa kapasite sınırsızdır.</p></label>
        </div>
      </section>

      {errors.form ? <p className="rounded-lg border border-red-300/20 bg-red-400/10 px-3 py-2 text-sm text-red-100" role="alert">{errors.form}</p> : null}
      <div className="flex flex-wrap items-center justify-end gap-3"><Button type="button" variant="ghost" onClick={() => router.push("/events")}>İptal</Button><Button type="submit" size="lg" disabled={pending}>{pending ? "Oluşturuluyor…" : "Etkinlik oluştur"}</Button></div>
    </form>
  );
}
