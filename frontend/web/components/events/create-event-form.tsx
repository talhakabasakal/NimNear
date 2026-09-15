"use client";

import { CalendarDays, Clock3, MapPin, Ticket, Users } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import {
  clearAuthSession,
  fetchCurrentUser,
  readAuthSession,
  type AuthSession,
  writeAuthSession,
} from "@/lib/api/auth";
import { AuthPanel } from "@/components/auth/auth-panel";
import { createEvent, EventsApiError, type CreateEventInput } from "@/lib/api/events";

type FormValues = {
  title: string;
  description: string;
  startsAt: string;
  endsAt: string;
  city: string;
  address: string;
  priceNim: string;
  capacity: string;
  latitude: string;
  longitude: string;
};

type FormErrors = Partial<Record<keyof FormValues, string>> & { form?: string };
type AuthStatus = "checking" | "anonymous" | "authenticated";

const initialValues: FormValues = {
  title: "",
  description: "",
  startsAt: "",
  endsAt: "",
  city: "İstanbul",
  address: "",
  priceNim: "",
  capacity: "",
  latitude: "",
  longitude: "",
};

const inputClass = "mt-2 h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30";
const labelClass = "text-xs font-medium text-foreground";

function FieldError({ message }: { message?: string }) {
  return message ? <p className="mt-1 text-xs text-red-200">{message}</p> : null;
}

function validateForm(values: FormValues, isFree: boolean): FormErrors {
  const errors: FormErrors = {};
  if (!values.title.trim()) errors.title = "Etkinlik adı gerekli.";
  if (values.title.trim().length > 255) errors.title = "Etkinlik adı 255 karakteri geçemez.";
  if (!values.startsAt) errors.startsAt = "Başlangıç zamanı gerekli.";
  if (!values.endsAt) errors.endsAt = "Bitiş zamanı gerekli.";

  const startsAt = values.startsAt ? new Date(values.startsAt) : null;
  const endsAt = values.endsAt ? new Date(values.endsAt) : null;
  if (startsAt && Number.isNaN(startsAt.getTime())) errors.startsAt = "Geçerli bir başlangıç zamanı seç.";
  if (endsAt && Number.isNaN(endsAt.getTime())) errors.endsAt = "Geçerli bir bitiş zamanı seç.";
  if (startsAt && endsAt && !Number.isNaN(startsAt.getTime()) && !Number.isNaN(endsAt.getTime()) && endsAt <= startsAt) errors.endsAt = "Bitiş zamanı başlangıçtan sonra olmalı.";

  if (!values.city.trim()) errors.city = "Şehir gerekli.";
  if (values.city.trim().length > 120) errors.city = "Şehir 120 karakteri geçemez.";
  if (values.address.trim().length > 500) errors.address = "Adres 500 karakteri geçemez.";
  if (values.description.length > 5000) errors.description = "Açıklama 5000 karakteri geçemez.";
  if (!isFree && values.priceNim.trim() && !/^[0-9]+(?:\.[0-9]{1,8})?$/.test(values.priceNim.trim())) errors.priceNim = "Fiyat 0 veya daha büyük, en fazla 8 ondalık basamaklı olmalı.";
  if (values.capacity.trim() && (!/^[0-9]+$/.test(values.capacity.trim()) || Number(values.capacity) < 1)) errors.capacity = "Kapasite pozitif bir tam sayı olmalı.";

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

export function CreateEventForm() {
  const router = useRouter();
  const [authStatus, setAuthStatus] = useState<AuthStatus>("checking");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [values, setValues] = useState<FormValues>(initialValues);
  const [isFree, setIsFree] = useState(true);
  const [errors, setErrors] = useState<FormErrors>({});
  const [pending, setPending] = useState(false);

  useEffect(() => {
    const stored = readAuthSession();
    if (!stored) {
      setAuthStatus("anonymous");
      return;
    }

    fetchCurrentUser(stored.token).then((user) => {
      const refreshed = { ...stored, user };
      writeAuthSession(refreshed);
      setSession(refreshed);
      setAuthStatus("authenticated");
    }).catch(() => {
      clearAuthSession();
      setAuthStatus("anonymous");
    });
  }, []);

  function updateValue(key: keyof FormValues, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: undefined, form: undefined }));
  }

  function handleAuthenticated(nextSession: AuthSession) {
    setSession(nextSession);
    setAuthStatus("authenticated");
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!session) return;

    const nextErrors = validateForm(values, isFree);
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      return;
    }

    const input: CreateEventInput = {
      title: values.title.trim(),
      description: values.description,
      starts_at: new Date(values.startsAt).toISOString(),
      ends_at: new Date(values.endsAt).toISOString(),
      price_nim: isFree ? "0" : values.priceNim.trim() || "0",
      currency: "NIM",
      city: values.city.trim(),
    };
    if (values.address.trim()) input.address = values.address.trim();
    if (values.capacity.trim()) input.capacity = Number(values.capacity);
    if (values.latitude.trim() && values.longitude.trim()) {
      input.latitude = Number(values.latitude);
      input.longitude = Number(values.longitude);
    }

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
        setErrors({ form: "Oturumun sona ermiş. Tekrar giriş yapmalısın." });
      } else {
        setErrors({ form: error instanceof EventsApiError ? error.message : "Etkinlik oluşturulamadı. Tekrar deneyebilirsin." });
      }
    } finally {
      setPending(false);
    }
  }

  if (authStatus === "checking") return <div className="h-48 animate-pulse rounded-xl border border-border bg-surface" aria-label="Kimlik doğrulama kontrol ediliyor" />;
  if (authStatus === "anonymous") return <AuthPanel onAuthenticated={handleAuthenticated} />;

  return (
    <form className="space-y-6" onSubmit={handleSubmit}>
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-surface px-4 py-3"><div><p className="text-xs text-muted">Oturum açıldı</p><p className="text-sm font-medium text-foreground">{session?.user.first_name} {session?.user.last_name} <span className="font-normal text-muted">· {session?.user.email}</span></p></div><button type="button" className="text-xs font-medium text-muted transition-colors hover:text-foreground" onClick={() => { clearAuthSession(); setSession(null); setAuthStatus("anonymous"); }}>Çıkış yap</button></div>

      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="mb-5"><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Temel bilgiler</p><h2 className="mt-1 text-xl font-semibold text-foreground">Etkinliğini tanımla</h2></div>
        <div className="space-y-5">
          <label className={labelClass}>Etkinlik adı<input className={inputClass} value={values.title} onChange={(event) => updateValue("title", event.target.value)} placeholder="Örn. Galata'da akşam buluşması" maxLength={255} /> <FieldError message={errors.title} /></label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className={labelClass}><span className="inline-flex items-center gap-2"><CalendarDays size={14} className="text-accent" />Başlangıç</span><input type="datetime-local" className={inputClass} value={values.startsAt} onChange={(event) => updateValue("startsAt", event.target.value)} /> <FieldError message={errors.startsAt} /></label>
            <label className={labelClass}><span className="inline-flex items-center gap-2"><Clock3 size={14} className="text-accent" />Bitiş</span><input type="datetime-local" className={inputClass} value={values.endsAt} onChange={(event) => updateValue("endsAt", event.target.value)} /> <FieldError message={errors.endsAt} /></label>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className={labelClass}><span className="inline-flex items-center gap-2"><MapPin size={14} className="text-accent" />Şehir</span><input className={inputClass} value={values.city} onChange={(event) => updateValue("city", event.target.value)} maxLength={120} /> <FieldError message={errors.city} /></label>
            <label className={labelClass}>Adres<input className={inputClass} value={values.address} onChange={(event) => updateValue("address", event.target.value)} placeholder="Örn. Galata Kulesi çevresi" maxLength={500} /> <FieldError message={errors.address} /></label>
          </div>
          <label className={labelClass}>Açıklama<textarea className="mt-2 min-h-32 w-full resize-y rounded-lg border border-border bg-background px-3 py-3 text-sm leading-6 text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30" value={values.description} onChange={(event) => updateValue("description", event.target.value)} placeholder="Etkinlik hakkında kısa bilgi" maxLength={5000} /> <FieldError message={errors.description} /></label>
        </div>
      </section>

      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="mb-5"><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Katılım</p><h2 className="mt-1 text-xl font-semibold text-foreground">Bilet ve kapasite</h2></div>
        <div className="grid gap-4 sm:grid-cols-2">
          <div><div className="flex items-center justify-between gap-3"><label className={labelClass} htmlFor="priceNim"><span className="inline-flex items-center gap-2"><Ticket size={14} className="text-accent" />NIM fiyatı</span></label><label className="inline-flex items-center gap-2 text-xs text-muted"><input type="checkbox" checked={isFree} onChange={(event) => { setIsFree(event.target.checked); setErrors((current) => ({ ...current, priceNim: undefined })); }} /> Ücretsiz</label></div><input id="priceNim" inputMode="decimal" className={`${inputClass} disabled:cursor-not-allowed disabled:opacity-50`} value={values.priceNim} onChange={(event) => updateValue("priceNim", event.target.value)} placeholder="0.00000000" disabled={isFree} /> <FieldError message={errors.priceNim} /></div>
          <label className={labelClass}><span className="inline-flex items-center gap-2"><Users size={14} className="text-accent" />Kapasite</span><input inputMode="numeric" className={inputClass} value={values.capacity} onChange={(event) => updateValue("capacity", event.target.value)} placeholder="İsteğe bağlı" /> <FieldError message={errors.capacity} /></label>
        </div>
        <p className="mt-4 text-xs leading-5 text-muted">Etkinlik, mevcut API tarafından herkese açık olarak oluşturulur. Tema, manzara ve takvim seçimi henüz API’ye bağlı değil.</p>
      </section>

      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <p className="mb-4 text-xs font-medium uppercase tracking-[0.16em] text-accent">İsteğe bağlı konum koordinatları</p>
        <div className="grid gap-4 sm:grid-cols-2"><label className={labelClass}>Enlem<input inputMode="decimal" className={inputClass} value={values.latitude} onChange={(event) => updateValue("latitude", event.target.value)} placeholder="41.0082" /> <FieldError message={errors.latitude} /></label><label className={labelClass}>Boylam<input inputMode="decimal" className={inputClass} value={values.longitude} onChange={(event) => updateValue("longitude", event.target.value)} placeholder="28.9784" /> <FieldError message={errors.longitude} /></label></div>
      </section>

      {errors.form ? <p className="rounded-lg border border-red-300/20 bg-red-400/10 px-3 py-2 text-sm text-red-100">{errors.form}</p> : null}
      <div className="flex flex-wrap items-center justify-end gap-3"><Button type="button" variant="ghost" onClick={() => router.push("/events")}>İptal</Button><Button type="submit" size="lg" disabled={pending}>{pending ? "Oluşturuluyor…" : "Etkinlik oluştur"}</Button></div>
    </form>
  );
}
