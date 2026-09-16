import Link from "next/link";
import { Bell, CalendarDays, Compass, Moon, Sparkles } from "lucide-react";

const navigation = [
  { href: "/", label: "Keşfet", icon: Compass },
  { href: "/events", label: "Etkinlikler", icon: CalendarDays },
  { href: "/calendars", label: "Takvimler", icon: CalendarDays },
  { href: "/events/create", label: "Etkinlik oluştur" },
];


export function AppHeader() {
  return (
    <header className="sticky top-0 z-20 border-b border-border/80 bg-background/90 backdrop-blur-xl">
      <div className="mx-auto flex h-[53px] max-w-[1240px] items-center gap-4 px-4 sm:px-6 lg:px-8">
        <Link href="/" className="flex shrink-0 items-center gap-2" aria-label="NIMNear ana sayfa">
          <span className="grid size-8 place-items-center rounded-lg bg-primary text-white"><Sparkles size={16} strokeWidth={2.2} /></span>
          <span className="hidden text-[15px] font-semibold tracking-[-0.02em] sm:inline">NIMNear</span>
        </Link>

        <nav aria-label="Ana navigasyon" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
          {navigation.map(({ href, label, icon: Icon }) => (
            <Link key={href} href={href} className="inline-flex h-9 shrink-0 items-center gap-2 rounded-lg px-3 text-[13px] font-medium text-muted transition-colors hover:bg-surface-hover hover:text-foreground">
              {Icon ? <Icon size={15} /> : null}{label}
            </Link>
          ))}
        </nav>

        <div className="flex shrink-0 items-center gap-1">
          <button type="button" className="grid size-9 place-items-center rounded-lg text-muted transition-colors hover:bg-surface-hover hover:text-foreground" aria-label="Tema değiştirme yakında" disabled><Moon size={16} /></button>
          <button type="button" className="relative grid size-9 place-items-center rounded-lg text-muted transition-colors hover:bg-surface-hover hover:text-foreground" aria-label="Bildirimler yakında" disabled><Bell size={16} /></button>
          <Link href="/profile" className="ml-1 grid size-8 place-items-center rounded-full bg-avatar text-[12px] font-semibold text-white transition-opacity hover:opacity-85" aria-label="Profil">N</Link>
        </div>
      </div>
    </header>
  );
}
