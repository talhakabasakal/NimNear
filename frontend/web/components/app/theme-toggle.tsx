"use client";

import { Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";

const themeStorageKey = "nimnear.theme";
type Theme = "dark" | "light";

function applyTheme(theme: Theme, persist: boolean) {
  document.documentElement.dataset.theme = theme;
  if (!persist) return;
  try {
    window.localStorage.setItem(themeStorageKey, theme);
  } catch {
    // The theme still applies when storage is unavailable.
  }
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>("dark");

  useEffect(() => {
    const mediaQuery = window.matchMedia("(prefers-color-scheme: light)");

    function syncTheme() {
      const current = document.documentElement.dataset.theme;
      setTheme(current === "light" ? "light" : "dark");
    }

    function followSystemTheme(event: MediaQueryListEvent) {
      try {
        const storedTheme = window.localStorage.getItem(themeStorageKey);
        if (storedTheme === "light" || storedTheme === "dark") return;
      } catch {
        // Continue with the system preference when storage is unavailable.
      }
      const nextTheme = event.matches ? "light" : "dark";
      applyTheme(nextTheme, false);
      setTheme(nextTheme);
    }

    syncTheme();
    mediaQuery.addEventListener("change", followSystemTheme);
    return () => mediaQuery.removeEventListener("change", followSystemTheme);
  }, []);

  function toggleTheme() {
    const nextTheme = theme === "dark" ? "light" : "dark";
    applyTheme(nextTheme, true);
    setTheme(nextTheme);
  }

  const nextTheme = theme === "dark" ? "light" : "dark";

  return (
    <button
      type="button"
      className="grid size-11 place-items-center rounded-lg text-muted transition-colors hover:bg-surface-hover hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      aria-label={`Switch to ${nextTheme} theme`}
      title={`Switch to ${nextTheme} theme`}
      onClick={toggleTheme}
    >
      {theme === "dark" ? <Sun size={16} aria-hidden="true" /> : <Moon size={16} aria-hidden="true" />}
    </button>
  );
}
