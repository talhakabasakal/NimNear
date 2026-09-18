import { AppHeader } from "@/components/app/app-header";
import { LoadingState } from "@/components/app/loading-state";

export default function Loading() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] space-y-6 px-4 py-8 sm:px-6 lg:px-8">
        <div className="h-10 w-52 animate-pulse rounded-lg bg-surface" />
        <LoadingState count={4} />
      </main>
    </div>
  );
}
