"use client";

import { useEffect, useState, type FormEvent } from "react";

import { StateCard } from "@/components/app/state-card";
import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { Button } from "@/components/ui/button";
import { createCalendar, fetchMyCalendars, type CalendarRecord, type MyCalendarsRecord } from "@/lib/api/calendars";
import { readAuthSession } from "@/lib/api/auth";

import { CalendarCard } from "./calendar-card";

const inputClass = "w-full rounded-lg border border-border bg-background px-3 py-2.5 text-sm text-foreground outline-none placeholder:text-muted focus:border-ring focus:ring-2 focus:ring-ring/30";

function CalendarGroup({ title, calendars, blocked = false }: { title: string; calendars: CalendarRecord[]; blocked?: boolean }) {
  return (
    <section className="space-y-3">
      <h2 className="text-sm font-semibold text-foreground">{title}</h2>
      {blocked ? <StateCard kind="empty" title="Backend oturumu gerekli" description="Kişisel takvimleri görmek için mevcut bir NIMNear backend oturumu gerekli." /> : calendars.length === 0 ? <StateCard kind="empty" title="Henüz takvim yok" description="Bu bölümde görünecek gerçek takvim bulunmuyor." /> : <div className="grid gap-3 sm:grid-cols-2">{calendars.map((calendar) => <CalendarCard key={calendar.id} calendar={calendar} />)}</div>}
    </section>
  );
}

export function CalendarWorkspace() {
  const [state, setState] = useState<"checking" | "anonymous" | "authenticated" | "error">("checking");
  const [token, setToken] = useState<string | null>(null);
  const [data, setData] = useState<MyCalendarsRecord>({ owned: [], followed: [] });
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<"public" | "private">("public");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  async function loadMine(currentToken: string) {
    try {
      setData(await fetchMyCalendars(currentToken));
      setState("authenticated");
    } catch {
      setState("error");
    }
  }

  useEffect(() => {
    const session = readAuthSession();
    if (!session) {
      setState("anonymous");
      return;
    }
    setToken(session.token);
    void loadMine(session.token);
  }, []);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || creating) return;
    setCreating(true);
    setCreateError(null);
    try {
      await createCalendar({ name, description, visibility }, token);
      setName("");
      setDescription("");
      await loadMine(token);
    } catch {
      setCreateError("Takvim oluşturulamadı. Alanları kontrol edip tekrar deneyebilirsin.");
    } finally {
      setCreating(false);
    }
  }

  if (state === "checking") return <div className="h-64 animate-pulse rounded-xl border border-border bg-surface" aria-label="Takvimlerin yükleniyor" />;
  if (state === "error") return <StateCard kind="error" title="Takvimlerin yüklenemedi" description="Kişisel takvim servisine şu anda ulaşılamıyor." />;
  if (state === "anonymous") {
    return <div className="space-y-8"><NimiqConnect description="Takvim oluşturmak veya takip ettiklerini görmek için Nimiq Pay hesabını bağlayabilirsin." blockedMessage="Nimiq hesabı bağlantısı backend oturumu oluşturmaz. Takvim sahipliği ve takip işlemleri, imza tabanlı NIMNear oturumu hazır olana kadar kullanılamaz." /><CalendarGroup title="Takvimlerim" calendars={[]} blocked /><CalendarGroup title="Takip edilenler" calendars={[]} blocked /></div>;
  }

  return (
    <div className="space-y-8">
      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Takvim oluştur</p><h2 className="mt-1 text-xl font-semibold text-foreground">Topluluğun için yeni bir takvim</h2></div>
        </div>
        <form className="mt-5 grid gap-3 sm:grid-cols-2" onSubmit={(event) => { void handleCreate(event); }}>
          <label className="text-xs text-muted">Takvim adı<input className={inputClass + " mt-2"} value={name} onChange={(event) => setName(event.target.value)} required maxLength={255} placeholder="Örn. Şehir buluşmaları" /></label>
          <label className="text-xs text-muted">Görünürlük<select className={inputClass + " mt-2"} value={visibility} onChange={(event) => setVisibility(event.target.value as "public" | "private")}><option value="public">Herkese açık</option><option value="private">Özel</option></select></label>
          <label className="text-xs text-muted sm:col-span-2">Açıklama<textarea className={inputClass + " mt-2 min-h-24 resize-y"} value={description} onChange={(event) => setDescription(event.target.value)} maxLength={5000} placeholder="Takvimin ne hakkında?" /></label>
          <div className="sm:col-span-2"><Button type="submit" disabled={creating}>{creating ? "Oluşturuluyor…" : "Oluştur"}</Button>{createError ? <p className="mt-2 text-xs text-red-200" role="alert">{createError}</p> : null}</div>
        </form>
      </section>
      <CalendarGroup title="Takvimlerim" calendars={data.owned} />
      <CalendarGroup title="Takip edilenler" calendars={data.followed} />
    </div>
  );
}
