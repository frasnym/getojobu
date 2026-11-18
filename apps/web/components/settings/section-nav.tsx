"use client";

import React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { UserRound } from "lucide-react";

export function SectionNav() {
  const pathname = usePathname();
  const isAccountActive = pathname?.startsWith("/settings/account");

  return (
    <nav aria-label="Settings sections" className="flex flex-col gap-1 text-sm font-medium">
      <Link
        href="/settings/account"
        aria-current={isAccountActive ? "page" : undefined}
        className={`flex items-center gap-2 rounded-lg px-3 py-2 transition-colors ${
          isAccountActive
            ? "bg-secondary text-foreground font-semibold"
            : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
        }`}
      >
        <UserRound className="h-4 w-4" />
        <span>Account</span>
      </Link>
    </nav>
  );
}
