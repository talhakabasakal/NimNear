import { AppHeader } from "@/components/app/app-header";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto w-full max-w-[760px] space-y-6 px-4 py-6 sm:px-6 sm:py-10 lg:px-8" aria-label="Loading wallet">
        <div className="space-y-2"><Skeleton className="h-3 w-24" /><Skeleton className="h-8 w-32" /></div>
        <div className="flex items-center gap-3"><Skeleton className="size-12 rounded-full" /><div className="space-y-2"><Skeleton className="h-3 w-24" /><Skeleton className="h-4 w-36" /></div></div>
        <Card><CardContent className="space-y-5 p-5 sm:p-6"><Skeleton className="h-3 w-28" /><Skeleton className="h-10 w-56 max-w-full" /><div className="grid grid-cols-2 gap-3"><Skeleton className="h-11" /><Skeleton className="h-11" /></div></CardContent></Card>
        <Skeleton className="h-6 w-40" />
        <Card><CardContent className="space-y-4"><Skeleton className="h-16 w-full" /><Skeleton className="h-16 w-full" /><Skeleton className="h-16 w-full" /></CardContent></Card>
      </main>
    </div>
  );
}
