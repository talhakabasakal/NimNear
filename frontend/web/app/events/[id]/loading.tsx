import { AppHeader } from "@/components/app/app-header";

export default function Loading() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] space-y-6 px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <div className="h-4 w-24 animate-pulse rounded bg-surface" />
        <div className="min-h-56 animate-pulse rounded-xl border border-border bg-surface sm:min-h-72" />
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-4"><div className="h-4 w-24 animate-pulse rounded bg-surface" /><div className="h-12 w-4/5 animate-pulse rounded bg-surface" /><div className="h-36 animate-pulse rounded-xl bg-surface" /></div>
          <div className="h-72 animate-pulse rounded-xl border border-border bg-surface" />
        </div>
      </main>
    </div>
  );
}
