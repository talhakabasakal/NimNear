import type { LucideIcon } from "lucide-react";

type CategoryCardProps = {
  label: string;
  icon: LucideIcon;
  tone: string;
};

export function CategoryCard({ label, icon: Icon, tone }: CategoryCardProps) {
  return (
    <div className="group rounded-xl border border-border bg-surface p-4 transition-colors hover:bg-surface-hover">
      <span className={"mb-5 grid size-9 place-items-center rounded-lg " + tone}><Icon size={17} /></span>
      <p className="text-sm font-medium text-foreground">{label}</p>
      <p className="mt-1 text-xs text-muted">İstanbul’da keşfet</p>
    </div>
  );
}
