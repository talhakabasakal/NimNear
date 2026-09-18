"use client";

import { useEffect, useState, type FormEvent } from "react";

import { StateCard } from "@/components/app/state-card";
import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { archiveCalendar, createCalendar, CalendarsApiError, fetchMyCalendars, updateCalendar, type CalendarRecord, type MyCalendarsRecord } from "@/lib/api/calendars";
import { userFacingCaughtError } from "@/lib/api/http-error";
import { clearAuthSession, isLostSessionStatus, readAuthSession } from "@/lib/api/auth";

import { CalendarCard } from "./calendar-card";

const inputClass = "w-full rounded-lg border border-border bg-background px-3 py-2.5 text-sm text-foreground outline-none placeholder:text-muted focus:border-ring focus:ring-2 focus:ring-ring/30";

function CalendarGroup({ title, calendars, blocked = false }: { title: string; calendars: CalendarRecord[]; blocked?: boolean }) {
  return (
    <section className="space-y-3">
      <h2 className="text-sm font-semibold text-foreground">{title}</h2>
      {blocked ? <StateCard kind="empty" title="Backend session required" description="An active NIMNear backend session is required to view personal calendars." /> : calendars.length === 0 ? <StateCard kind="empty" title="No calendars yet" description="There are no calendars to show in this section." /> : <div className="grid gap-3 sm:grid-cols-2">{calendars.map((calendar) => <CalendarCard key={calendar.id} calendar={calendar} />)}</div>}
    </section>
  );
}

function OwnedCalendarCard({
  calendar,
  onUpdated,
}: {
  calendar: CalendarRecord;
  onUpdated: () => Promise<void>;
}) {
  const [name, setName] = useState(calendar.name);
  const [saving, setSaving] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [confirmArchive, setConfirmArchive] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const archived = calendar.status === "archived";

  async function handleRename(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (archived || saving || name.trim() === calendar.name) return;
    setSaving(true);
    setError(null);
    try {
      await updateCalendar(calendar.id, { name: name.trim() });
      await onUpdated();
    } catch (error) {
      setError(userFacingCaughtError(error, "The calendar could not be updated."));
    } finally {
      setSaving(false);
    }
  }

  async function handleArchive() {
    if (archived || archiving) return;
    setArchiving(true);
    setError(null);
    try {
      await archiveCalendar(calendar.id);
      setConfirmArchive(false);
      await onUpdated();
    } catch (error) {
      setError(userFacingCaughtError(error, "The calendar could not be archived."));
    } finally {
      setArchiving(false);
    }
  }

  return (
    <article className="space-y-3">
      <CalendarCard calendar={calendar} />
      {archived ? (
        <p className="text-xs leading-5 text-muted">Archived calendars stay in your workspace and keep their events. They are hidden from public discovery and cannot receive new events.</p>
      ) : (
        <form className="space-y-3" onSubmit={(event) => { void handleRename(event); }}>
          <label className="text-xs text-muted">Name<input className={inputClass + " mt-2"} value={name} onChange={(event) => setName(event.target.value)} required maxLength={255} /></label>
          <div className="flex flex-wrap gap-2">
            <Button type="submit" size="sm" disabled={saving || name.trim() === calendar.name}>{saving ? "Saving…" : "Save name"}</Button>
            <Button type="button" size="sm" variant="outline" onClick={() => { setError(null); setConfirmArchive(true); }}>Archive</Button>
          </div>
        </form>
      )}
      {error ? <p className="text-xs text-red-200" role="alert">{error}</p> : null}
      <ConfirmDialog
        open={confirmArchive}
        title="Archive this calendar?"
        description="It will leave public discovery. Existing events stay attached and are not deleted."
        confirmLabel="Archive calendar"
        pending={archiving}
        pendingLabel="Archiving…"
        error={error}
        onConfirm={() => {
          void handleArchive();
        }}
        onOpenChange={setConfirmArchive}
      />
    </article>
  );
}

export function CalendarWorkspace() {
  const [state, setState] = useState<"checking" | "anonymous" | "authenticated" | "error">("checking");
  const [authenticated, setAuthenticated] = useState(false);
  const [data, setData] = useState<MyCalendarsRecord>({ owned: [], followed: [] });
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<"public" | "private">("public");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  async function loadMine() {
    try {
      setData(await fetchMyCalendars());
      setAuthenticated(true);
      setState("authenticated");
    } catch (error) {
      if (error instanceof CalendarsApiError && isLostSessionStatus(error.status)) {
        clearAuthSession();
        setAuthenticated(false);
        setState("anonymous");
        return;
      }
      setState("error");
    }
  }

  useEffect(() => {
    const session = readAuthSession();
    if (!session) {
      setState("anonymous");
      return;
    }
    void loadMine();
  }, []);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!authenticated || creating) return;
    setCreating(true);
    setCreateError(null);
    try {
      await createCalendar({ name, description, visibility });
      setName("");
      setDescription("");
      await loadMine();
    } catch (error) {
      setCreateError(userFacingCaughtError(error, "The calendar could not be created. Check the fields and try again."));
    } finally {
      setCreating(false);
    }
  }

  if (state === "checking") return <div className="h-64 animate-pulse rounded-xl border border-border bg-surface" aria-label="Loading calendars" />;
  if (state === "error") return <StateCard kind="error" title="Your calendars could not be loaded" description="The personal calendar service is currently unavailable." />;
  if (state === "anonymous") {
    return <div className="space-y-8"><NimiqConnect description="Sign in securely with your Nimiq wallet to create a calendar or view the calendars you follow." /><CalendarGroup title="My calendars" calendars={[]} blocked /><CalendarGroup title="Following" calendars={[]} blocked /></div>;
  }

  return (
    <div className="space-y-8">
      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Create a calendar</p><h2 className="mt-1 text-xl font-semibold text-foreground">A new calendar for your community</h2></div>
        </div>
        <form className="mt-5 grid gap-3 sm:grid-cols-2" onSubmit={(event) => { void handleCreate(event); }}>
          <label className="text-xs text-muted">Calendar name<input className={inputClass + " mt-2"} value={name} onChange={(event) => setName(event.target.value)} required maxLength={255} placeholder="e.g. City meetups" /></label>
          <label className="text-xs text-muted">Visibility<select className={inputClass + " mt-2"} value={visibility} onChange={(event) => setVisibility(event.target.value as "public" | "private")}><option value="public">Public</option><option value="private">Private</option></select></label>
          <label className="text-xs text-muted sm:col-span-2">Description<textarea className={inputClass + " mt-2 min-h-24 resize-y"} value={description} onChange={(event) => setDescription(event.target.value)} maxLength={5000} placeholder="What is this calendar about?" /></label>
          <div className="sm:col-span-2"><Button type="submit" disabled={creating}>{creating ? "Creating…" : "Create"}</Button>{createError ? <p className="mt-2 text-xs text-red-200" role="alert">{createError}</p> : null}</div>
        </form>
      </section>
      <section className="space-y-3">
        <h2 className="text-sm font-semibold text-foreground">My calendars</h2>
        {data.owned.length === 0 ? <StateCard kind="empty" title="No calendars yet" description="There are no calendars to show in this section." /> : <div className="grid gap-3 sm:grid-cols-2">{data.owned.map((calendar) => <OwnedCalendarCard key={calendar.id + calendar.updated_at} calendar={calendar} onUpdated={loadMine} />)}</div>}
      </section>
      <CalendarGroup title="Following" calendars={data.followed} />
    </div>
  );
}
