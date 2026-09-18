"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { accountDisplayName, readAuthSession } from "@/lib/api/auth";

export function ProfileButton() {
  const [label, setLabel] = useState("N");

  useEffect(() => {
    function sync() {
      const session = readAuthSession();
      if (!session) {
        setLabel("N");
        return;
      }
      const name = accountDisplayName(session.user);
      setLabel(Array.from(name)[0]?.toUpperCase() || "N");
    }
    sync();
    window.addEventListener("nimnear-auth-changed", sync);
    return () => window.removeEventListener("nimnear-auth-changed", sync);
  }, []);

  return (
    <Link
      href="/profile"
      className="ml-1 grid size-11 place-items-center rounded-full bg-avatar text-[12px] font-semibold text-white transition-opacity hover:opacity-85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      aria-label="Profile"
    >
      {label}
    </Link>
  );
}
