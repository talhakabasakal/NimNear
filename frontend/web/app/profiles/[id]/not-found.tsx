import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";

export default function NotFound() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1000px] px-4 py-8 sm:px-6 lg:px-8">
        <StateCard kind="empty" title="Profile not found" description="This profile does not exist or is no longer public." />
        <Link href="/" className="mt-5 block text-center text-xs font-medium text-accent hover:text-foreground">
          Back to Discover
        </Link>
      </main>
    </div>
  );
}
