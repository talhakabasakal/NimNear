"use client";

import { useEffect } from "react";
import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";

export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <div className="mx-auto max-w-[760px] px-4 py-10 sm:px-6">
        <StateCard
          kind="error"
          title="Something went wrong"
          description="This page could not be loaded. Try again, or return to Events."
        />
        <div className="mt-5 flex flex-wrap justify-center gap-3">
          <button
            type="button"
            className="inline-flex h-11 items-center rounded-lg bg-primary px-4 text-sm font-medium text-white"
            onClick={() => reset()}
          >
            Try again
          </button>
          <Link href="/" className="inline-flex h-11 items-center rounded-lg px-4 text-sm font-medium text-muted hover:text-foreground">
            Back to Events
          </Link>
        </div>
      </div>
    </div>
  );
}
