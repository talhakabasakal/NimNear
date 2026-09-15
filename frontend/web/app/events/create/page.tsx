import { ArrowLeft, Plus } from "lucide-react";
import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { CreateEventForm } from "@/components/events/create-event-form";

export default function CreateEventPage() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[960px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <Link href="/events" className="mb-6 inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Etkinlikler</Link>
        <header className="mb-8 flex items-start gap-3">
          <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/15 text-accent"><Plus size={19} /></span>
          <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">İstanbul</p><h1 className="mt-1 text-3xl font-semibold tracking-[-0.035em] text-foreground">Etkinlik oluştur</h1><p className="mt-2 max-w-2xl text-sm leading-6 text-muted">Topluluğun için yeni bir buluşma planla.</p></div>
        </header>
        <CreateEventForm />
      </main>
    </div>
  );
}
