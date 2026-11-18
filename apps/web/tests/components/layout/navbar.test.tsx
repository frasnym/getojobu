import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import React from "react";
import { Navbar } from "@/components/layout/navbar";

vi.mock("@clerk/nextjs", () => ({
  UserButton: () => <div data-testid="clerk-user-button">UserButton</div>,
  SignedIn: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SignedOut: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SignInButton: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/user-jobs",
}));

vi.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: "dark", setTheme: vi.fn() }),
}));

describe("Navbar component", () => {
  it("renders the getojobu branding and navigation links", () => {
    render(<Navbar />);
    expect(screen.getByText("getojobu")).toBeInTheDocument();
    expect(screen.getByText("My Jobs")).toBeInTheDocument();
    expect(screen.getByText("Explore Jobs")).toBeInTheDocument();
    expect(screen.getByTestId("clerk-user-button")).toBeInTheDocument();
  });

  it("renders the theme toggle in the navbar", () => {
    render(<Navbar />);
    expect(screen.getByRole("button", { name: /switch to light theme/i })).toBeInTheDocument();
  });
});
