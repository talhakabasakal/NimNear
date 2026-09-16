"use client";

import { CalendarDays, UserRound } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { ExternalMediaImage } from "@/components/media/external-media-image";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { EventCard } from "@/components/events/event-card";
import { fetchProfile, fetchProfileEvents, type ProfileRecord } from "@/lib/api/profiles";
import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { readAuthSession } from "@/lib/api/auth";
import type { EventRecord } from "@/lib/api/events";

type ProfileScreenProps = {
  profileId?: string;
  initialProfile?: ProfileRecord;
};

const joinedFormatter = new Intl.DateTimeFormat("tr-TR", { month: "long", year: "numeric" });

function profileName(profile: ProfileRecord) {
  return profile.display_name.trim() || "NIMNear kullanıcısı";
}

function ProfileAvatar({ profile }: { profile: ProfileRecord }) {
  return <ExternalMediaImage src={profile.avatar_url} className="size-20 rounded-full border border-border object-cover sm:size-24" fallback={<span className="grid size-20 place-items-center rounded-full bg-avatar text-white sm:size-24"><UserRound size={30} /></span>} />;
}

function EventSection({ title, events, emptyDescription }: { title: string; events: EventRecord[]; emptyDescription: string }) {
  return (
    <section className="space-y-4">
      <div>
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Etkinlikler</p>
        <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">{title}</h2>
      </div>
      {events.length === 0 ? <StateCard kind="empty" title="Henüz etkinlik yok" description={emptyDescription} /> : <div className="grid gap-4 md:grid-cols-2">{events.map((event) => <EventCard key={event.id} event={event} compact />)}</div>}
    </section>
  );
}

function ProfileContent({ profile, organized, attended, showCreateAction }: { profile: ProfileRecord; organized: EventRecord[]; attended: EventRecord[]; showCreateAction: boolean }) {
  const [activeTab, setActiveTab] = useState<"organized" | "attended">("organized");
  const activeEvents = activeTab === "organized" ? organized : attended;
  const activeTitle = activeTab === "organized" ? "Düzenlediği etkinlikler" : "Katıldığı etkinlikler";
  const activeDescription = activeTab === "organized" ? "Bu kullanıcı henüz public bir etkinlik düzenlemedi." : "Bu kullanıcı henüz bir etkinliğe katılmadı.";
  const joinedAt = useMemo(() => {
    const date = new Date(profile.joined_at);
    return Number.isNaN(date.getTime()) ? "Tarih bilgisi yok" : joinedFormatter.format(date);
  }, [profile.joined_at]);

  return (
    <main className="mx-auto max-w-[1000px] space-y-8 px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
      <section className="rounded-xl border border-border bg-surface p-5 sm:p-7">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <ProfileAvatar profile={profile} />
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Profil</p>
            <h1 className="mt-1 truncate text-2xl font-semibold tracking-[-0.03em] text-foreground sm:text-[28px]">{profileName(profile)}</h1>
            {profile.username ? <p className="mt-1 text-sm text-muted">@{profile.username}</p> : null}
            {profile.bio ? <p className="mt-3 max-w-xl whitespace-pre-wrap text-sm leading-6 text-muted">{profile.bio}</p> : null}
            <p className="mt-3 flex items-center gap-2 text-xs text-muted"><CalendarDays size={14} className="text-accent" /> NIMNear&apos;a {joinedAt} tarihinde katıldı</p>
          </div>
        </div>
        {showCreateAction ? <div className="mt-5"><Link href="/profile/edit" className="inline-flex h-10 items-center rounded-lg border border-border bg-transparent px-4 text-sm font-medium text-foreground transition-colors hover:bg-surface-hover">Profili düzenle</Link></div> : null}
        <div className="mt-6 grid max-w-md grid-cols-2 gap-3 border-t border-border pt-5">
          <div><p className="text-2xl font-semibold text-foreground">{profile.organized_event_count}</p><p className="mt-1 text-xs text-muted">Düzenlenen etkinlik</p></div>
          <div><p className="text-2xl font-semibold text-foreground">{profile.attended_event_count}</p><p className="mt-1 text-xs text-muted">Katılınan etkinlik</p></div>
        </div>
      </section>

      <div className="flex w-fit rounded-lg border border-border bg-surface p-1" role="tablist" aria-label="Profil etkinlikleri">
        <button type="button" role="tab" aria-selected={activeTab === "organized"} onClick={() => setActiveTab("organized")} className={"rounded-md px-3 py-2 text-xs font-medium " + (activeTab === "organized" ? "bg-surface-hover text-foreground" : "text-muted")}>Düzenledikleri</button>
        <button type="button" role="tab" aria-selected={activeTab === "attended"} onClick={() => setActiveTab("attended")} className={"rounded-md px-3 py-2 text-xs font-medium " + (activeTab === "attended" ? "bg-surface-hover text-foreground" : "text-muted")}>Katıldıkları</button>
      </div>

      <EventSection title={activeTitle} events={activeEvents} emptyDescription={activeDescription} />
      {showCreateAction && activeTab === "organized" && organized.length === 0 ? <Link href="/events/create" className="inline-flex h-10 items-center rounded-lg bg-primary px-4 text-sm font-medium text-white transition-colors hover:opacity-90">İlk etkinliğini oluştur</Link> : null}
      {showCreateAction ? <NimiqConnect description="Nimiq Pay hesabını bağlayabilirsin. Bu bağlantı henüz NIMNear oturumu oluşturmaz." blockedMessage="Profil ve etkinlik geçmişi için Nimiq imzası ile backend oturumu oluşturma desteği bekleniyor." /> : null}
    </main>
  );
}

export function ProfileScreen({ profileId, initialProfile }: ProfileScreenProps) {
  const [profile, setProfile] = useState<ProfileRecord | null>(initialProfile ?? null);
  const [organized, setOrganized] = useState<EventRecord[]>([]);
  const [attended, setAttended] = useState<EventRecord[]>([]);
  const [loading, setLoading] = useState(!initialProfile);
  const [needsAuth, setNeedsAuth] = useState(false);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const currentSession = readAuthSession();
      if (!profileId && !currentSession) {
        if (!cancelled) { setNeedsAuth(true); setLoading(false); }
        return;
      }
      const id = profileId ?? currentSession?.user.id;
      if (!id) return;

      try {
        const [loadedProfile, organizedEvents, attendedEvents] = await Promise.all([
          initialProfile ?? fetchProfile(id),
          fetchProfileEvents(id, "organized"),
          fetchProfileEvents(id, "attended"),
        ]);
        if (!cancelled) {
          setProfile(loadedProfile);
          setOrganized(organizedEvents);
          setAttended(attendedEvents);
        }
      } catch {
        if (!cancelled) setError(true);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => { cancelled = true; };
  }, [initialProfile, profileId]);

  if (needsAuth) {
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[760px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><NimiqConnect description="Nimiq Pay hesabını bağlayabilirsin. Bu bağlantı henüz NIMNear oturumu oluşturmaz." blockedMessage="Profil ve etkinlik geçmişi için Nimiq imzası ile backend oturumu oluşturma desteği bekleniyor." /></main></div>;
  }
  if (loading) {
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1000px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><div className="h-64 animate-pulse rounded-xl border border-border bg-surface" /></main></div>;
  }
  if (error || !profile) {
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1000px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><StateCard kind="error" title="Profil yüklenemedi" description="Profil servisine şu anda ulaşılamıyor veya profil bulunamadı." /></main></div>;
  }

  return <div className="min-h-svh bg-background"><AppHeader /><ProfileContent profile={profile} organized={organized} attended={attended} showCreateAction={!profileId} /></div>;
}
