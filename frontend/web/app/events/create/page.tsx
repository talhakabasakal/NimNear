import { AppHeader } from "@/components/app/app-header";
import { CreateEventForm } from "@/components/events/create-event-form";

export default function CreateEventPage() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main>
        <CreateEventForm />
      </main>
    </div>
  );
}
