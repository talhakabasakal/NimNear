import { AppHeader } from "@/components/app/app-header";

export default function Loading() {
  return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1120px] space-y-6 px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><div className="h-4 w-24 animate-pulse rounded bg-surface" /><div className="h-80 animate-pulse rounded-xl border border-border bg-surface" /><div className="h-64 animate-pulse rounded-xl border border-border bg-surface" /></main></div>;
}
