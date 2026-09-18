"use client";

import {
  CalendarDays,
  Check,
  ChevronDown,
  Clock3,
  FileText,
  Globe2,
  Image as ImageIcon,
  LayoutTemplate,
  Link2,
  MapPin,
  Palette,
  PenLine,
  Shuffle,
  Ticket,
  Users,
} from "lucide-react";
import { useEffect, useMemo, useState, type CSSProperties, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { Button } from "@/components/ui/button";
import { ExternalMediaImage } from "@/components/media/external-media-image";
import {
  AuthApiError,
  clearAuthSession,
  fetchCurrentUser,
  isLostSessionStatus,
  readAuthSession,
  type AuthSession,
  writeAuthSession,
} from "@/lib/api/auth";
import { CalendarsApiError, fetchMyCalendars, type CalendarRecord } from "@/lib/api/calendars";
import { createEvent, EventsApiError, normalizeNimPrice, type CreateEventInput } from "@/lib/api/events";
import { userFacingCaughtError } from "@/lib/api/http-error";
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

type EventTheme = {
  id: string;
  name: string;
  description: string;
  background: string;
  backgroundStrong: string;
  panel: string;
  field: string;
  line: string;
  accent: string;
  foreground: string;
  muted: string;
};

const eventThemes: EventTheme[] = [
  { id: "minimal", name: "Minimal", description: "Soft, focused and timeless", background: "#4c1d49", backgroundStrong: "#32152f", panel: "#6d3e69", field: "#5e315b", line: "#895480", accent: "#f5b6eb", foreground: "#fff7ff", muted: "#d9b7d5" },
  { id: "quantum", name: "Quantum", description: "Electric gradients for big ideas", background: "#202058", backgroundStrong: "#15163e", panel: "#36358b", field: "#2d2c76", line: "#5a59b2", accent: "#a9e7ff", foreground: "#f7f9ff", muted: "#c1c7ed" },
  { id: "warp", name: "Warp", description: "A dark canvas with neon energy", background: "#19152d", backgroundStrong: "#0d0b1a", panel: "#2e2450", field: "#241b43", line: "#4f3b82", accent: "#edb4ff", foreground: "#fbf8ff", muted: "#bdb2d8" },
  { id: "emoji", name: "Emoji", description: "Bright, playful and expressive", background: "#6d4d87", backgroundStrong: "#4f3567", panel: "#8663a0", field: "#76538f", line: "#b18bc4", accent: "#ffe49b", foreground: "#fffaff", muted: "#ecdaf2" },
  { id: "confetti", name: "Confetti", description: "A little celebration in every detail", background: "#551d78", backgroundStrong: "#35124e", panel: "#742ca0", field: "#64228a", line: "#a350c7", accent: "#ffc6f7", foreground: "#fff8ff", muted: "#e1b7ed" },
  { id: "pattern", name: "Pattern", description: "Playful geometry, clean structure", background: "#3c2369", backgroundStrong: "#251543", panel: "#54358a", field: "#482b78", line: "#795bb0", accent: "#d6c4ff", foreground: "#faf8ff", muted: "#c7bce4" },
  { id: "seasonal", name: "Seasonal", description: "Warm colors for memorable moments", background: "#7a3212", backgroundStrong: "#4b1c0a", panel: "#9a4a19", field: "#863b13", line: "#c66e36", accent: "#ffe0a8", foreground: "#fffaf2", muted: "#f0c8a4" },
  { id: "aurora", name: "Aurora", description: "Cool northern light for late-night ideas", background: "#123d57", backgroundStrong: "#092534", panel: "#1d6877", field: "#17556a", line: "#4295a0", accent: "#a5f6d8", foreground: "#f1fffc", muted: "#b4dedb" },
  { id: "ocean", name: "Ocean", description: "Clear, calm and open to everyone", background: "#103a52", backgroundStrong: "#092538", panel: "#165d75", field: "#124d66", line: "#3987a2", accent: "#a6e2ff", foreground: "#f3fbff", muted: "#b7d8e7" },
  { id: "forest", name: "Forest", description: "Grounded greens for a slower rhythm", background: "#1d4938", backgroundStrong: "#102b23", panel: "#2e704f", field: "#255c43", line: "#4b946b", accent: "#c0f3aa", foreground: "#f5fff2", muted: "#b6d8b4" },
  { id: "sunset", name: "Sunset", description: "Golden-hour energy for shared moments", background: "#812d4b", backgroundStrong: "#4d1833", panel: "#ad4b48", field: "#963b45", line: "#d66e5e", accent: "#ffd1a3", foreground: "#fff8f2", muted: "#f2b9a6" },
  { id: "candy", name: "Candy", description: "Sweet, bright and unapologetically fun", background: "#702558", backgroundStrong: "#431738", panel: "#a34387", field: "#8c3473", line: "#cc6fb0", accent: "#ffc8f2", foreground: "#fff8fd", muted: "#edb9df" },
  { id: "mono", name: "Mono", description: "Quiet contrast and modern restraint", background: "#30343b", backgroundStrong: "#1e2126", panel: "#555c66", field: "#464d57", line: "#737d89", accent: "#f4f7fa", foreground: "#ffffff", muted: "#c8d0d8" },
  { id: "retro", name: "Retro", description: "A warm throwback with a little glow", background: "#663e22", backgroundStrong: "#3c2416", panel: "#966531", field: "#805329", line: "#be8a49", accent: "#ffdc86", foreground: "#fff9e9", muted: "#e3c48d" },
];

const inputClass = "mt-2 h-11 w-full rounded-xl border px-3 text-sm outline-none transition focus:ring-2";
const labelClass = "text-xs font-medium";

function FieldError({ message, id }: { message?: string; id?: string }) {
  return message ? <p id={id} className="mt-1 text-xs text-red-200" role="alert">{message}</p> : null;
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
  if (!values.title.trim()) errors.title = "Event title is required.";
  if (values.title.trim().length > 255) errors.title = "Event title cannot exceed 255 characters.";
  if (!values.startsAt) errors.startsAt = "Start time is required.";
  if (!values.endsAt) errors.endsAt = "End time is required.";

  const startsAt = serializeDateTimeLocal(values.startsAt);
  const endsAt = serializeDateTimeLocal(values.endsAt);
  if (values.startsAt && !startsAt) errors.startsAt = "Select a valid start time.";
  if (values.endsAt && !endsAt) errors.endsAt = "Select a valid end time.";
  if (startsAt && endsAt && new Date(endsAt) <= new Date(startsAt)) errors.endsAt = "End time must be after the start time.";

  if (!values.city.trim()) errors.city = "City is required.";
  if (values.city.trim().length > 120) errors.city = "City cannot exceed 120 characters.";
  if (values.address.trim().length > 500) errors.address = "Address cannot exceed 500 characters.";
  if (values.description.length > 5000) errors.description = "Description cannot exceed 5,000 characters.";
  if (!isFree) {
    const normalizedPrice = normalizeNimPrice(values.priceNim);
    if (!values.priceNim.trim()) errors.priceNim = "A NIM price is required for paid events.";
    else if (!normalizedPrice) errors.priceNim = "The price must be a NIM amount with at most 5 decimal places and no rounding required.";
    else if (normalizedPrice === "0") errors.priceNim = "Use the Free option for a free event.";
  }
  if (!values.capacity.trim()) {
    // Empty capacity is the explicit unlimited-capacity state.
  } else if (!/^[0-9]+$/.test(values.capacity.trim()) || !Number.isSafeInteger(Number(values.capacity)) || Number(values.capacity) < 1 || Number(values.capacity) > 2147483647) {
    errors.capacity = "Capacity must be an integer between 1 and 2,147,483,647.";
  }
  if (!isValidMediaURL(values.imageURL.trim())) errors.imageURL = "Image must be an absolute HTTP(S) URL no longer than 2,048 characters.";

  const hasLatitude = Boolean(values.latitude.trim());
  const hasLongitude = Boolean(values.longitude.trim());
  if (hasLatitude !== hasLongitude) {
    errors.latitude = "Latitude and longitude must be entered together.";
    errors.longitude = "Latitude and longitude must be entered together.";
  } else if (hasLatitude && hasLongitude) {
    const latitude = Number(values.latitude);
    const longitude = Number(values.longitude);
    if (!Number.isFinite(latitude) || latitude < -90 || latitude > 90) errors.latitude = "Latitude must be between -90 and 90.";
    if (!Number.isFinite(longitude) || longitude < -180 || longitude > 180) errors.longitude = "Longitude must be between -180 and 180.";
  }
  return errors;
}

function browserLocationError(code: number) {
  if (code === 1) return "Location permission was denied.";
  if (code === 2) return "Location could not be retrieved.";
  if (code === 3) return "The location request timed out.";
  return "Location could not be retrieved.";
}


function ThemeArtwork({ theme, compact = false }: { theme: EventTheme; compact?: boolean }) {
  const artworkStyle: CSSProperties = {
    background: `radial-gradient(circle at 25% 20%, ${theme.accent} 0, transparent 25%), radial-gradient(circle at 88% 84%, ${theme.panel} 0, transparent 30%), linear-gradient(135deg, ${theme.backgroundStrong}, ${theme.background} 52%, ${theme.panel})`,
  };
  return (
    <div className={`relative isolate size-full overflow-hidden ${compact ? "rounded-lg" : "rounded-[inherit]"}`} style={artworkStyle} aria-hidden="true">
      <span className="absolute -left-[14%] top-[18%] h-[48%] w-[62%] rotate-[25deg] rounded-[46%] border-[10px] border-white/20" />
      <span className="absolute -right-[12%] bottom-[4%] h-[64%] w-[58%] -rotate-[28deg] rounded-[48%] border-[12px] border-white/15" />
      <span className="absolute right-[18%] top-[13%] size-5 rounded-full bg-white/70 shadow-[0_0_28px_8px_rgba(255,255,255,0.22)]" />
      <span className="absolute bottom-[16%] left-[20%] size-10 rounded-full border-4 border-white/25" />
    </div>
  );
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
  const [selectedThemeId, setSelectedThemeId] = useState("minimal");
  const [themeReady, setThemeReady] = useState(false);
  const selectedTheme = useMemo(() => eventThemes.find((theme) => theme.id === selectedThemeId) ?? eventThemes[0], [selectedThemeId]);

  useEffect(() => {
    try {
      const storedTheme = window.localStorage.getItem("nimnear.event-theme");
      if (storedTheme && eventThemes.some((theme) => theme.id === storedTheme)) setSelectedThemeId(storedTheme);
    } catch {
      // The default theme still works when storage is unavailable.
    } finally {
      setThemeReady(true);
    }
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.eventPage = "true";
    root.dataset.eventTheme = selectedTheme.id;
    const rootThemeVariables = {
      "--event-background": selectedTheme.background,
      "--event-background-strong": selectedTheme.backgroundStrong,
      "--event-panel": selectedTheme.panel,
      "--event-field": selectedTheme.field,
      "--event-line": selectedTheme.line,
      "--event-accent": selectedTheme.accent,
      "--event-foreground": selectedTheme.foreground,
      "--event-muted": selectedTheme.muted,
    };
    Object.entries(rootThemeVariables).forEach(([property, value]) => root.style.setProperty(property, value));
    if (themeReady) try {
      window.localStorage.setItem("nimnear.event-theme", selectedTheme.id);
    } catch {
      // The selected theme still applies for the current session.
    }
    return () => {
      delete root.dataset.eventPage;
      delete root.dataset.eventTheme;
      Object.keys(rootThemeVariables).forEach((property) => root.style.removeProperty(property));
    };
  }, [selectedTheme, themeReady]);

  useEffect(() => {
    let cancelled = false;
    const stored = readAuthSession();
    if (!stored) {
      setAuthStatus("anonymous");
      return () => { cancelled = true; };
    }

    fetchCurrentUser().then((user) => {
      if (cancelled) return;
      const refreshed = { ...stored, user };
      writeAuthSession(refreshed);
      setSession(refreshed);
      setAuthStatus("authenticated");
    }).catch((error: unknown) => {
      if (cancelled) return;
      if (error instanceof AuthApiError && isLostSessionStatus(error.status)) {
        clearAuthSession();
        setAuthStatus("anonymous");
        return;
      }
      setAuthError(error instanceof Error ? error.message : "Session status could not be retrieved.");
      setAuthStatus("error");
    });
    return () => { cancelled = true; };
  }, [retryKey]);

  useEffect(() => {
    if (authStatus !== "authenticated" || !session) return;
    let cancelled = false;
    setCalendarStatus("loading");
    setCalendarError(null);
    fetchMyCalendars().then((result) => {
      if (cancelled) return;
      setOwnedCalendars(result.owned.filter((calendar) => calendar.status !== "archived"));
      setCalendarStatus("ready");
    }).catch((error: unknown) => {
      if (cancelled) return;
      if (error instanceof CalendarsApiError && error.status === 401) {
        clearAuthSession();
        setSession(null);
        setAuthStatus("anonymous");
        return;
      }
      setCalendarError(error instanceof Error ? error.message : "Calendars could not be loaded.");
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
      setPlaceError("This environment does not support location access.");
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
          setPlaceError(error instanceof Error ? error.message : "Nearby places could not be loaded.");
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
      setErrors({ form: "Event details are invalid. Check the fields." });
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
      const created = await createEvent(input);
      router.push(`/events/${created.id}`);
    } catch (error) {
      if (error instanceof EventsApiError && error.status === 401) {
        clearAuthSession();
        setSession(null);
        setAuthStatus("anonymous");
        setErrors({ form: "Your session has expired. Sign in again with your Nimiq wallet." });
      } else {
        setErrors({ form: userFacingCaughtError(error, "The event could not be created. Try again.") });
      }
    } finally {
      setPending(false);
    }
  }

  const themeStyle = {
    "--event-background": selectedTheme.background,
    "--event-background-strong": selectedTheme.backgroundStrong,
    "--event-panel": selectedTheme.panel,
    "--event-field": selectedTheme.field,
    "--event-line": selectedTheme.line,
    "--event-accent": selectedTheme.accent,
    "--event-foreground": selectedTheme.foreground,
    "--event-muted": selectedTheme.muted,
  } as CSSProperties;

  if (authStatus === "checking") return <div className="create-event-shell min-h-[calc(100svh-53px)] px-4 py-8" style={themeStyle}><div className="mx-auto h-48 max-w-[1080px] animate-pulse rounded-3xl border border-white/10 bg-white/10" aria-label="Checking authentication" /></div>;
  if (authStatus === "error") return <div className="create-event-shell min-h-[calc(100svh-53px)] px-4 py-8" style={themeStyle}><section className="mx-auto max-w-[620px] rounded-3xl border border-red-200/20 bg-red-400/10 p-6"><p className="text-sm font-medium" style={{ color: "var(--event-foreground)" }}>Session status could not be loaded.</p><p className="mt-1 text-xs leading-5" style={{ color: "var(--event-muted)" }}>{authError ?? "The identity service is currently unavailable."}</p><Button type="button" variant="outline" className="mt-4" onClick={() => { setAuthError(null); setAuthStatus("checking"); setRetryKey((value) => value + 1); }}>Try again</Button></section></div>;
  if (authStatus === "anonymous") return <div className="create-event-shell min-h-[calc(100svh-53px)] px-4 py-8" style={themeStyle}><div className="mx-auto max-w-[620px]"><NimiqConnect description="Sign in with your Nimiq wallet to create an event." /></div></div>;

  const selectedPlace = places.find((place) => place.id === selectedPlaceId) ?? null;
  const previewImage = isValidMediaURL(values.imageURL.trim()) ? values.imageURL.trim() : null;

  return (
    <div className="create-event-shell min-h-[calc(100svh-53px)]" style={themeStyle}>
      <div className="mx-auto max-w-[1080px] px-4 pb-16 pt-6 sm:px-6 lg:px-8">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="mb-2 inline-flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--event-accent)" }}><Palette size={13} /> Event studio</p>
            <h1 className="text-3xl font-semibold tracking-[-0.045em] sm:text-[42px]" style={{ color: "var(--event-foreground)" }}>Create an event</h1>
            <p className="mt-2 max-w-xl text-sm leading-6" style={{ color: "var(--event-muted)" }}>Shape the details, choose a mood and give your community a place to meet.</p>
          </div>
        </div>

        <form className="grid gap-5 lg:grid-cols-[minmax(280px,340px)_minmax(0,1fr)] lg:items-start" onSubmit={handleSubmit}>
          <aside className="space-y-5 lg:sticky lg:top-[77px]">
            <section className="create-event-panel rounded-3xl border p-3 sm:p-4">
              <div className="relative aspect-square overflow-hidden rounded-2xl">
                <ExternalMediaImage src={previewImage} alt="" className="size-full object-cover" fallback={<ThemeArtwork theme={selectedTheme} />} />
                <label htmlFor="imageURL" className="absolute bottom-3 right-3 inline-flex size-10 cursor-pointer items-center justify-center rounded-full border-2 border-black/20 bg-white text-[#2c172d] shadow-lg transition hover:scale-105" aria-label="Change cover image"><ImageIcon size={18} /></label>
                <div className="absolute left-3 top-3 rounded-full bg-black/25 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-white backdrop-blur">Live preview</div>
              </div>
              <div className="mt-3 flex items-center justify-between gap-3"><div><p className="text-xs font-medium" style={{ color: "var(--event-foreground)" }}>Cover image</p><p className="mt-0.5 text-[11px]" style={{ color: "var(--event-muted)" }}>Use a URL or let the theme lead.</p></div><ImageIcon size={16} style={{ color: "var(--event-accent)" }} /></div>
              <label className="sr-only" htmlFor="imageURL">Image URL</label>
              <div className="relative mt-3"><Link2 className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2" size={14} style={{ color: "var(--event-muted)" }} /><input id="imageURL" type="url" className={inputClass + " pl-9"} style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.imageURL} onChange={(event) => updateValue("imageURL", event.target.value)} placeholder="https://cover-image…" maxLength={2048} /> </div><FieldError message={errors.imageURL} />
            </section>

            <section className="create-event-panel rounded-3xl border p-4">
              <div className="mb-3 flex items-center justify-between"><div><p className="text-sm font-medium" style={{ color: "var(--event-foreground)" }}>Choose a theme</p><p className="mt-0.5 text-[11px]" style={{ color: "var(--event-muted)" }}>{selectedTheme.description}</p></div><button type="button" className="grid size-8 place-items-center rounded-lg transition hover:bg-white/10" style={{ color: "var(--event-muted)" }} aria-label="Choose a random theme" title="Choose a random theme" onClick={() => setSelectedThemeId(eventThemes[Math.floor(Math.random() * eventThemes.length)].id)}><Shuffle size={15} /></button></div>
              <div className="grid grid-cols-4 gap-2 sm:grid-cols-7 lg:grid-cols-4">
                {eventThemes.map((theme) => <button key={theme.id} type="button" className="group min-w-0 text-left" aria-pressed={selectedTheme.id === theme.id} onClick={() => setSelectedThemeId(theme.id)}><span className={"relative block aspect-square rounded-xl p-0.5 transition " + (selectedTheme.id === theme.id ? "ring-2 ring-white ring-offset-2 ring-offset-transparent" : "opacity-75 group-hover:opacity-100")}><ThemeArtwork theme={theme} compact />{selectedTheme.id === theme.id ? <span className="absolute right-1 top-1 grid size-4 place-items-center rounded-full bg-white text-[#3b1b3c]"><Check size={11} strokeWidth={3} /></span> : null}</span><span className="mt-1.5 block truncate text-[10px]" style={{ color: selectedTheme.id === theme.id ? "var(--event-foreground)" : "var(--event-muted)" }}>{theme.name}</span></button>)}
              </div>
            </section>
          </aside>

          <div className="space-y-5">
            <section className="create-event-panel rounded-3xl border p-5 sm:p-7">
              <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
                <label className="relative inline-flex items-center" style={{ color: "var(--event-muted)" }}><LayoutTemplate className="pointer-events-none absolute left-3" size={15} /><select aria-label="Calendar" className="h-9 appearance-none rounded-xl border pl-9 pr-8 text-xs font-medium outline-none" style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={selectedCalendarId} onChange={(event) => setSelectedCalendarId(event.target.value)} disabled={calendarStatus === "loading"}><option value="">Personal calendar</option>{ownedCalendars.map((calendar) => <option key={calendar.id} value={calendar.id}>{calendar.name}</option>)}</select><ChevronDown className="pointer-events-none absolute right-2.5" size={14} /></label>
                <div className="inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-xs" style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-muted)" }}><Globe2 size={14} /> Everyone can see this</div>
              </div>
              {calendarStatus === "loading" ? <p className="mb-4 text-xs" style={{ color: "var(--event-muted)" }} role="status">Loading your calendars…</p> : null}
              {calendarStatus === "error" ? <div className="mb-4 flex flex-wrap items-center gap-3 text-xs text-red-100"><span>{calendarError ?? "Calendars could not be loaded."} You can continue without selecting a calendar.</span><Button type="button" variant="outline" size="sm" onClick={() => setCalendarRetryKey((value) => value + 1)}>Try again</Button></div> : null}
              {calendarStatus === "ready" && ownedCalendars.length === 0 ? <p className="mb-4 text-xs" style={{ color: "var(--event-muted)" }}>You do not own any calendars yet. This event will be created without one.</p> : null}

              <label className="block text-[11px] font-semibold uppercase tracking-[0.16em]" style={{ color: "var(--event-muted)" }}>Event title<input className="mt-3 w-full border-0 bg-transparent p-0 text-3xl font-semibold tracking-[-0.04em] outline-none placeholder:text-white/35 focus:ring-0 sm:text-[40px]" style={{ color: "var(--event-foreground)" }} value={values.title} onChange={(event) => updateValue("title", event.target.value)} placeholder="Event name" maxLength={255} /></label><FieldError message={errors.title} />

              <div className="mt-7 grid gap-3 sm:grid-cols-2">
                <label className={labelClass} style={{ color: "var(--event-muted)" }}><span className="inline-flex items-center gap-2"><CalendarDays size={14} style={{ color: "var(--event-accent)" }} /> Starts</span><input type="datetime-local" className={inputClass} style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.startsAt} onChange={(event) => updateValue("startsAt", event.target.value)} /><FieldError message={errors.startsAt} /></label>
                <label className={labelClass} style={{ color: "var(--event-muted)" }}><span className="inline-flex items-center gap-2"><Clock3 size={14} style={{ color: "var(--event-accent)" }} /> Ends</span><input type="datetime-local" className={inputClass} style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.endsAt} onChange={(event) => updateValue("endsAt", event.target.value)} /><FieldError message={errors.endsAt} /></label>
              </div>

              <div className="mt-3 rounded-2xl border p-4" style={{ backgroundColor: "color-mix(in srgb, var(--event-field) 72%, transparent)", borderColor: "var(--event-line)" }}>
                <div className="flex items-center gap-2 text-xs font-medium" style={{ color: "var(--event-foreground)" }}><MapPin size={15} style={{ color: "var(--event-accent)" }} /> Location</div>
                <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,0.7fr)_minmax(0,1.3fr)]"><label className={labelClass} style={{ color: "var(--event-muted)" }}>City<input className={inputClass} style={{ backgroundColor: "var(--event-panel)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.city} onChange={(event) => updateValue("city", event.target.value)} placeholder="Istanbul" maxLength={120} /><FieldError message={errors.city} /></label><label className={labelClass} style={{ color: "var(--event-muted)" }}>Address or meeting point<input className={inputClass} style={{ backgroundColor: "var(--event-panel)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.address} onChange={(event) => updateValue("address", event.target.value)} placeholder="Add an address or online link" maxLength={500} disabled={Boolean(selectedPlace)} /><FieldError message={errors.address} /></label></div>
                <div className="mt-3 flex flex-wrap items-center justify-between gap-3"><p className="text-[11px]" style={{ color: "var(--event-muted)" }}>{selectedPlace ? selectedPlace.name + " · " + selectedPlace.address : "Offline location or a virtual meeting link"}</p><button type="button" className="inline-flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-[11px] font-semibold transition hover:bg-white/10" style={{ color: "var(--event-accent)" }} onClick={loadNearbyPlaceOptions} disabled={placeStatus === "loading"}>{placeStatus === "loading" ? "Loading places…" : "Use a saved place"}</button></div>
                {placeStatus === "error" ? <p className="mt-2 text-xs text-red-100" role="alert">{placeError}</p> : null}
                {placeStatus === "empty" ? <p className="mt-2 text-xs" style={{ color: "var(--event-muted)" }}>No saved places nearby. You can continue with a custom address.</p> : null}
                {places.length > 0 ? <label className={labelClass + " mt-3 block"} style={{ color: "var(--event-muted)" }}>Saved place<select className={inputClass} style={{ backgroundColor: "var(--event-panel)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={selectedPlaceId} onChange={(event) => selectPlace(event.target.value)}><option value="">Use custom address</option>{places.map((place) => <option key={place.id} value={place.id}>{place.name} · {Math.round(place.distance_meters)} m</option>)}</select></label> : null}
                <details className="mt-3 text-xs" style={{ color: "var(--event-muted)" }}><summary className="cursor-pointer select-none list-none font-medium"><span className="inline-flex items-center gap-2"><ChevronDown size={13} /> Add coordinates</span></summary><div className="mt-3 grid gap-3 sm:grid-cols-2"><label className={labelClass}>Latitude<input inputMode="decimal" className={inputClass} style={{ backgroundColor: "var(--event-panel)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.latitude} onChange={(event) => updateValue("latitude", event.target.value)} placeholder="41.0082" disabled={Boolean(selectedPlace)} /><FieldError message={errors.latitude} /></label><label className={labelClass}>Longitude<input inputMode="decimal" className={inputClass} style={{ backgroundColor: "var(--event-panel)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.longitude} onChange={(event) => updateValue("longitude", event.target.value)} placeholder="28.9784" disabled={Boolean(selectedPlace)} /><FieldError message={errors.longitude} /></label></div></details>
              </div>

              <label className="mt-3 block text-xs font-medium" style={{ color: "var(--event-muted)" }}><span className="inline-flex items-center gap-2"><FileText size={14} style={{ color: "var(--event-accent)" }} /> Description</span><textarea className="mt-2 min-h-24 w-full resize-y rounded-2xl border px-3 py-3 text-sm leading-6 outline-none transition focus:ring-2" style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.description} onChange={(event) => updateValue("description", event.target.value)} placeholder="What should people know before they join?" maxLength={5000} /><FieldError message={errors.description} /></label>
            </section>

            <section className="create-event-panel rounded-3xl border p-5 sm:p-7">
              <div className="mb-4 flex items-center justify-between"><div><p className="text-sm font-medium" style={{ color: "var(--event-foreground)" }}>Event options</p><p className="mt-1 text-xs" style={{ color: "var(--event-muted)" }}>Set the access and attendance details.</p></div><Ticket size={17} style={{ color: "var(--event-accent)" }} /></div>
              <div className="divide-y rounded-2xl border" style={{ borderColor: "var(--event-line)" }}>
                <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"><span className="inline-flex items-center gap-2 text-sm" style={{ color: "var(--event-foreground)" }}><Ticket size={15} style={{ color: "var(--event-accent)" }} /> Ticket price</span><div className="flex items-center gap-3"><label className="inline-flex items-center gap-2 text-xs" style={{ color: "var(--event-muted)" }}><input type="checkbox" checked={isFree} onChange={(event) => { setIsFree(event.target.checked); setErrors((current) => ({ ...current, priceNim: undefined })); }} /> Free</label><label className="sr-only" htmlFor="priceNim">NIM price</label><input id="priceNim" inputMode="decimal" aria-invalid={Boolean(errors.priceNim)} aria-describedby={errors.priceNim ? "priceNim-error" : undefined} className="h-9 w-28 rounded-lg border px-2 text-right text-sm outline-none disabled:opacity-50" style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.priceNim} onChange={(event) => updateValue("priceNim", event.target.value)} placeholder="0.00000 NIM" disabled={isFree} /></div></div>
                <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"><span className="inline-flex items-center gap-2 text-sm" style={{ color: "var(--event-foreground)" }}><Users size={15} style={{ color: "var(--event-accent)" }} /> Capacity</span><label className="sr-only" htmlFor="capacity">Capacity</label><input id="capacity" inputMode="numeric" aria-invalid={Boolean(errors.capacity)} aria-describedby={errors.capacity ? "capacity-error" : undefined} className="h-9 w-28 rounded-lg border px-2 text-right text-sm outline-none" style={{ backgroundColor: "var(--event-field)", borderColor: "var(--event-line)", color: "var(--event-foreground)" }} value={values.capacity} onChange={(event) => updateValue("capacity", event.target.value)} placeholder="Unlimited" /></div>
              </div>
              <div className="mt-2 flex flex-wrap justify-between gap-3 text-[11px]" style={{ color: "var(--event-muted)" }}><FieldError id="priceNim-error" message={errors.priceNim} /><FieldError id="capacity-error" message={errors.capacity} /><span>1 NIM = 100,000 Luna · blank capacity is unlimited</span></div>
            </section>

            {errors.form ? <p className="rounded-2xl border border-red-200/20 bg-red-400/10 px-4 py-3 text-sm text-red-100" role="alert">{errors.form}</p> : null}
            <div className="flex flex-wrap items-center justify-end gap-3 pb-4"><button type="button" className="rounded-xl px-4 py-3 text-sm font-medium transition hover:bg-white/10" style={{ color: "var(--event-muted)" }} onClick={() => router.push("/events")}>Cancel</button><button type="submit" className="inline-flex h-12 min-w-48 items-center justify-center gap-2 rounded-xl px-6 text-sm font-semibold transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-60" style={{ backgroundColor: "var(--event-foreground)", color: "var(--event-backgroundStrong)" }} disabled={pending}>{pending ? "Creating…" : "Create event"}<PenLine size={15} /></button></div>
          </div>
        </form>
      </div>
    </div>
  );
}
