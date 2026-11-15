import "@testing-library/jest-dom";
import React from "react";
import { vi } from "vitest";

// Mock Clerk useAuth and components for test suite
vi.mock("@clerk/nextjs", () => ({
  useAuth: () => ({
    getToken: vi.fn().mockResolvedValue("mock-clerk-test-token"),
    isLoaded: true,
    isSignedIn: true,
    userId: "user_test_123",
  }),
  UserButton: () =>
    React.createElement("div", { "data-testid": "clerk-user-button" }, "UserButton"),
  SignedIn: ({ children }: { children: React.ReactNode }) => children,
  SignedOut: ({ children }: { children: React.ReactNode }) => children,
  SignInButton: ({ children }: { children: React.ReactNode }) => children,
}));

