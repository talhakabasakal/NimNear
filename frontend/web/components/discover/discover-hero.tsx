import Link from "next/link";
import { ArrowRight, MapPin, Sparkles } from "lucide-react";

type DiscoverHeroProps = {
  eventCount: number;
  hasError?: boolean;
};

export function DiscoverHero({ eventCount, hasError = false }: DiscoverHeroProps) {
  return (
    <section className="relative isolate min-h-[300px] overflow-hidden rounded-xl border border-border bg-[#171522] px-6 py-8 sm:min-h-[360px] sm:px-10 sm:py-12">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(circle_at_76%_30%,rgba(215,166,255,0.28),transparent_26%),radial-gradient(circle_at_20%_0%,rgba(130,119,255,0.24),transparent_32%),linear-gradient(125deg,#191624,#121116_64%,#2a2036)]" />
      <div className="absolute -right-20 top-8 -z-10 size-72 rounded-full border border-accent/15" />
      <div className="absolute -right-8 top-20 -z-10 size-52 rounded-full border border-primary/15" />
      <div className="absolute bottom-0 left-0 right-0 -z-10 h-24 bg-[linear-gradient(165deg,transparent_25%,rgba(15,14,16,0.72)_26%,rgba(15,14,16,0.92)_100%)]" />
      <div className="absolute bottom-0 left-0 right-0 -z-10 flex h-20 items-end gap-1 opacity-55" aria-hidden="true">
        {[18, 28, 15, 38, 24, 48, 30, 21, 42, 26, 35, 19, 46, 29, 17, 34, 25, 41, 20, 31].map((height, index) => (
          <span key={index} className="flex-1 bg-[#0d0c10]" style={{ height: height + "px" }} />
        ))}
      </div>

      <div className="max-w-lg">
        <span className="mb-5 inline-flex items-center gap-2 rounded-full border border-accent/25 bg-accent/10 px-3 py-1.5 text-[11px] font-medium text-accent"><Sparkles size={13} /> NIMNear discovery</span>
        <h1 className="max-w-md text-3xl font-semibold leading-[1.05] tracking-[-0.04em] text-white sm:text-[42px]">Discover beautiful moments.</h1>
        <p className="mt-4 max-w-sm text-sm leading-6 text-white/65">Discover community events and new meetups in one place.</p>
        <div className="mt-7 flex flex-wrap items-center gap-3">
          <Link href="/events" className="inline-flex h-10 items-center gap-2 rounded-lg bg-primary px-4 text-sm font-semibold text-white transition-colors hover:bg-primary/90">Explore events <ArrowRight size={15} /></Link>
          <span className="inline-flex items-center gap-2 text-xs text-white/60"><MapPin size={14} />{hasError ? "Waiting for event service" : eventCount + " upcoming events"}</span>
        </div>
      </div>
    </section>
  );
}
