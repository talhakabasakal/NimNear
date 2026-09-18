import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";

export default function PayRequestNotFound() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[560px] px-4 py-8 sm:px-6 sm:py-10">
        <StateCard
          kind="empty"
          title="Payment request not found"
          description="This request does not exist or may have been removed."
        />
      </main>
    </div>
  );
}
