import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import React from "react";
import { ThemeProvider } from "@/components/theme-provider";
import { ThemeToggle } from "@/components/layout/theme-toggle";

describe("ThemeToggle component", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.className = "";
    document.documentElement.style.colorScheme = "";
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  it.each([
    ["dark", "light"],
    ["light", "dark"],
  ])("switches the document theme from %s to %s", async (initialTheme, expectedTheme) => {
    render(
      <ThemeProvider attribute="class" defaultTheme={initialTheme} enableSystem={false}>
        <ThemeToggle />
      </ThemeProvider>
    );

    await waitFor(() => expect(document.documentElement).toHaveClass(initialTheme));
    const toggle = screen.getByRole("button", {
      name: initialTheme === "dark" ? /switch to light theme/i : /switch to dark theme/i,
    });

    fireEvent.click(toggle);

    await waitFor(() => expect(document.documentElement).toHaveClass(expectedTheme));
    expect(
      screen.getByRole("button", {
        name: expectedTheme === "dark" ? /switch to light theme/i : /switch to dark theme/i,
      })
    ).toBeInTheDocument();
  });
});
