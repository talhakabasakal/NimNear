import { AlertCircle, Inbox } from "lucide-react";

type StateCardProps = {
  kind: "empty" | "error";
  title: string;
  description: string;
};

export function StateCard({ kind, title, description }: StateCardProps) {
  const Icon = kind === "error" ? AlertCircle : Inbox;

  return (
    <div
      role={kind === "error" ? "alert" : "status"}
      className="flex min-h-36 flex-col items-center justify-center rounded-xl border border-dashed border-border-faint bg-surface/60 px-6 text-center"
    >
      <span className="mb-3 grid size-9 place-items-center rounded-full bg-surface-hover text-muted"><Icon size={17} /></span>
      <p className="text-sm font-medium text-foreground">{title}</p>
      <p className="mt-1 max-w-sm text-xs leading-5 text-muted">{description}</p>
    </div>
  );
}
