import React from "react";
import Link from "next/link";
import { UserButton } from "@clerk/nextjs";
import { Briefcase, Sparkles } from "lucide-react";

export function Navbar() {
  return (
    <header className="sticky top-0 z-50 w-full border-b border-border/40 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
        <div className="flex items-center gap-6">
          <Link href="/" className="flex items-center gap-2 font-bold tracking-tight">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <Briefcase className="h-4 w-4" />
            </div>
            <span className="text-lg font-black tracking-tight">getojobu</span>
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              MVP
            </span>
          </Link>

          <nav className="flex items-center gap-4 text-sm font-medium">
            <Link
              href="/"
              className="flex items-center gap-1.5 text-foreground/80 transition-colors hover:text-foreground"
            >
              <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
              <span>Jobs & Matches</span>
            </Link>
          </nav>
        </div>

        <div className="flex items-center gap-4">
          <UserButton
            appearance={{
              elements: {
                avatarBox: "h-8 w-8",
              },
            }}
          />
        </div>
      </div>
    </header>
  );
}
