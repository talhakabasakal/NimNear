import { Skeleton } from "@/components/ui/skeleton";
import { AppHeader } from "@/components/app/app-header";

export default function PayRequestLoading() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto w-full max-w-[560px] px-4 py-6 sm:px-6 sm:py-10" aria-label="Loading payment request">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="mt-3 h-10 w-40" />
        <Skeleton className="mt-6 h-48 w-full" />
      </main>
    </div>
  );
}
