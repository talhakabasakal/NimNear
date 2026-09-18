import { ArrowLeft } from "lucide-react";
import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";

export default function NotFound() {
  return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1120px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><Link href="/" className="mb-6 inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Discover</Link><StateCard kind="empty" title="Place not found" description="This place does not exist or is no longer active." /></main></div>;
}
