"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

import { AppHeader } from "@/components/app/app-header";
import { AuthPanel } from "@/components/auth/auth-panel";
import { StateCard } from "@/components/app/state-card";
import { Button } from "@/components/ui/button";
import { readAuthSession } from "@/lib/api/auth";
import { fetchProfile, ProfilesApiError, updateProfile, type ProfileRecord } from "@/lib/api/profiles";

type FieldKey = "displayName" | "username" | "bio";
type FieldErrors = Partial<Record<FieldKey, string>> & { form?: string };
type FormValues = { displayName: string; username: string; bio: string };

const inputClass = "mt-2 h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30";
const labelClass = "text-xs font-medium text-foreground";

function profileName(profile: ProfileRecord) {
  return profile.display_name.trim() || "NIMNear kullanıcısı";
}

function fieldMessage(error: ProfilesApiError): FieldErrors {
  switch (error.errorCode) {
    case "invalid_display_name":
      return { displayName: "Görünen ad 1–100 karakter olmalı ve satır sonu içermemeli." };
    case "invalid_username":
      return { username: "Kullanıcı adı 3–30 karakter olmalı; yalnızca küçük harf, rakam, alt çizgi ve tire kullanabilir." };
    case "reserved_username":
      return { username: "Bu kullanıcı adı kullanılamıyor." };
    case "username_taken":
      return { username: "Bu kullanıcı adı zaten alınmış." };
    case "invalid_bio":
      return { bio: "Biyografide desteklenmeyen kontrol karakterleri var." };
    case "bio_too_long":
      return { bio: "Biyografi en fazla 280 karakter olabilir." };
    case "empty_profile_update":
      return { form: "Kaydetmek için en az bir alan değiştir." };
    default:
      return { form: error.message || "Profil güncellenemedi. Tekrar deneyebilirsin." };
  }
}

function ProfileEditForm({ initialProfile, token }: { initialProfile: ProfileRecord; token: string }) {
  const router = useRouter();
  const [values, setValues] = useState<FormValues>({
    displayName: initialProfile.display_name,
    username: initialProfile.username ?? "",
    bio: initialProfile.bio,
  });
  const [errors, setErrors] = useState<FieldErrors>({});
  const [pending, setPending] = useState(false);

  function updateValue(key: FieldKey, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: undefined, form: undefined }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setErrors({});

    try {
      await updateProfile({
        display_name: values.displayName.trim() || null,
        username: values.username.trim().toLowerCase() || null,
        bio: values.bio.replace(/\r\n?/g, "\n") || null,
      }, token);
      router.replace("/profile");
      router.refresh();
    } catch (error) {
      setErrors(error instanceof ProfilesApiError ? fieldMessage(error) : { form: "Profil güncellenemedi. Tekrar deneyebilirsin." });
    } finally {
      setPending(false);
    }
  }

  return (
    <section className="rounded-xl border border-border bg-surface p-5 sm:p-7">
      <div className="flex items-start gap-4 border-b border-border pb-5">
        {initialProfile.avatar_url ? <img src={initialProfile.avatar_url} alt="" className="size-16 rounded-full border border-border object-cover" /> : <span className="grid size-16 place-items-center rounded-full bg-avatar text-lg font-semibold text-white">N</span>}
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Profil</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-[-0.03em] text-foreground">Profili düzenle</h1>
          <p className="mt-2 text-sm leading-6 text-muted">{profileName(initialProfile)} için görünen bilgileri güncelle.</p>
        </div>
      </div>

      <form className="mt-6 space-y-5" onSubmit={handleSubmit}>
        <label className={labelClass}>
          Görünen ad
          <input className={inputClass} value={values.displayName} onChange={(event) => updateValue("displayName", event.target.value)} autoComplete="name" aria-invalid={Boolean(errors.displayName)} />
          {errors.displayName ? <p className="mt-1 text-xs text-red-200">{errors.displayName}</p> : null}
        </label>

        <label className={labelClass}>
          Kullanıcı adı
          <input className={inputClass} value={values.username} onChange={(event) => updateValue("username", event.target.value.toLowerCase())} maxLength={30} autoComplete="username" placeholder="kullanici_adi" aria-invalid={Boolean(errors.username)} />
          <p className="mt-1 text-xs text-muted">İsteğe bağlıdır. Küçük harf, rakam, alt çizgi ve tire kullanılabilir.</p>
          {errors.username ? <p className="mt-1 text-xs text-red-200">{errors.username}</p> : null}
        </label>

        <label className={labelClass}>
          Biyografi
          <textarea className="mt-2 min-h-32 w-full resize-y rounded-lg border border-border bg-background px-3 py-3 text-sm leading-6 text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30" value={values.bio} onChange={(event) => updateValue("bio", event.target.value)} aria-invalid={Boolean(errors.bio)} />
          <span className="mt-1 block text-right text-xs text-muted">{Array.from(values.bio).length}/280</span>
          {errors.bio ? <p className="mt-1 text-xs text-red-200">{errors.bio}</p> : null}
        </label>

        <div className="rounded-lg border border-border bg-background/50 px-3 py-3 text-xs leading-5 text-muted">
          Avatar, e-posta ve hesap kimlik bilgileri bu sürümde değiştirilemez.
        </div>

        {errors.form ? <p className="rounded-lg border border-red-300/20 bg-red-400/10 px-3 py-2 text-sm text-red-100">{errors.form}</p> : null}

        <div className="flex flex-wrap gap-3 pt-1">
          <Button type="submit" disabled={pending}>{pending ? "Kaydediliyor…" : "Değişiklikleri kaydet"}</Button>
          <Button type="button" variant="outline" disabled={pending} onClick={() => router.push("/profile")}>Vazgeç</Button>
        </div>
      </form>
    </section>
  );
}

export default function ProfileEditPage() {
  const [profile, setProfile] = useState<ProfileRecord | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [needsAuth, setNeedsAuth] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const session = readAuthSession();
      if (!session) {
        if (!cancelled) {
          setNeedsAuth(true);
          setLoading(false);
        }
        return;
      }

      try {
        const loadedProfile = await fetchProfile(session.user.id);
        if (!cancelled) {
          setProfile(loadedProfile);
          setToken(session.token);
        }
      } catch {
        if (!cancelled) setError(true);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void load();
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[760px] space-y-5 px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <Link href="/profile" className="inline-flex text-sm text-muted transition-colors hover:text-foreground">← Profile dön</Link>
        {needsAuth ? <AuthPanel title="Profili düzenlemek için giriş yap" description="Profil bilgilerin doğrulanmış hesabınla düzenlenir." onAuthenticated={() => window.location.reload()} /> : null}
        {loading ? <div className="h-[500px] animate-pulse rounded-xl border border-border bg-surface" /> : null}
        {!loading && error ? <StateCard kind="error" title="Profil yüklenemedi" description="Profil servisine şu anda ulaşılamıyor. Tekrar deneyebilirsin." /> : null}
        {!loading && !error && profile && token ? <ProfileEditForm initialProfile={profile} token={token} /> : null}
      </main>
    </div>
  );
}
