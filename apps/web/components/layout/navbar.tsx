"use client";

import React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { UserButton } from "@clerk/nextjs";
import { Briefcase, Sparkles, Compass } from "lucide-react";
import { ThemeToggle } from "@/components/layout/theme-toggle";

export function Navbar() {
  const pathname = usePathname();

  const isUserJobsActive = pathname?.startsWith("/user-jobs");
  const isJobsActive = pathname === "/jobs" || (pathname?.startsWith("/jobs") && !isUserJobsActive);

  return (
    <header className="sticky top-0 z-40 w-full border-b border-border/40 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
        <div className="flex items-center gap-6">
          <Link href="/user-jobs" className="flex items-center gap-2 font-bold tracking-tight">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <Briefcase className="h-4 w-4" />
            </div>
            <span className="text-lg font-black tracking-tight">getojobu</span>
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              MVP
            </span>
          </Link>

          <nav className="flex items-center gap-1 sm:gap-2 text-sm font-medium">
            <Link
              href="/user-jobs"
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-colors ${
                isUserJobsActive
                  ? "bg-secondary text-foreground font-semibold"
                  : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
              }`}
            >
              <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
              <span>My Jobs</span>
            </Link>

            <Link
              href="/jobs"
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-colors ${
                isJobsActive
                  ? "bg-secondary text-foreground font-semibold"
                  : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
              }`}
            >
              <Compass className="h-3.5 w-3.5 text-sky-500" />
              <span>Explore Jobs</span>
            </Link>
          </nav>
        </div>

        <div className="flex items-center gap-4">
          <ThemeToggle />
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
