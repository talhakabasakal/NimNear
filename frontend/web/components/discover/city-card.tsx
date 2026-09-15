import { ArrowUpRight, MapPin } from "lucide-react";

type CityCardProps = {
  name: string;
  detail?: string;
  featured?: boolean;
};

export function CityCard({ name, detail = "Etkinlikleri keşfet", featured = false }: CityCardProps) {
  return (
    <div className={"group relative overflow-hidden rounded-xl border border-border bg-surface " + (featured ? "min-h-48 p-6" : "min-h-28 p-4")}>
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_85%_15%,rgba(215,166,255,0.22),transparent_32%),linear-gradient(145deg,rgba(130,119,255,0.12),transparent_60%)]" />
      <div className="relative flex h-full flex-col justify-between gap-8">
        <span className="flex size-9 items-center justify-center rounded-lg bg-primary/15 text-accent"><MapPin size={16} /></span>
        <div className="flex items-end justify-between gap-3">
          <div><p className="text-lg font-semibold text-foreground">{name}</p><p className="mt-1 text-xs text-muted">{detail}</p></div>
          <ArrowUpRight size={17} className="text-muted transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-accent" />
        </div>
      </div>
    </div>
  );
}
