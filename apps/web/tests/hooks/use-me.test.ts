import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useMe } from "@/hooks/use-me";
import { getMe, updateMe, deactivateMe } from "@/lib/api/endpoints";
import { AccountUser } from "@/lib/api/types";

const { getToken, signOut } = vi.hoisted(() => ({
  getToken: vi.fn().mockResolvedValue("test-token"),
  signOut: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("@clerk/nextjs", () => ({
  useAuth: () => ({
    getToken,
    isLoaded: true,
    signOut,
  }),
}));

vi.mock("@/lib/api/endpoints", () => ({
  getMe: vi.fn(),
  updateMe: vi.fn(),
  deactivateMe: vi.fn(),
}));

const sampleUser: AccountUser = {
  id: "user-1",
  email: "jane@example.com",
  full_name: "Jane Doe",
  headline: "Backend Engineer",
  phone: "+62 812",
  portfolio_url: "https://jane.dev",
  linkedin_url: "https://linkedin.com/in/jane",
  github_url: "https://github.com/jane",
  role: "user",
  onboarding_completed_at: "2026-11-01T00:00:00Z",
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-11-01T00:00:00Z",
};

describe("useMe hook", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getMe).mockResolvedValue(sampleUser);
    vi.mocked(updateMe).mockResolvedValue(sampleUser);
    vi.mocked(deactivateMe).mockResolvedValue(sampleUser);
  });

  it("Test useMe loads the current user", async () => {
    const { result } = renderHook(() => useMe());

    await waitFor(() => {
      expect(result.current.user?.id).toBe("user-1");
    });

    expect(result.current.form.full_name).toBe("Jane Doe");
  });

  it("Test useMe.save sends all six fields", async () => {
    vi.mocked(updateMe).mockResolvedValue({ ...sampleUser, full_name: "New Name" });
    const { result } = renderHook(() => useMe());

    await waitFor(() => {
      expect(result.current.user?.id).toBe("user-1");
    });

    act(() => {
      result.current.setField("full_name", "New Name");
    });
    expect(result.current.form.full_name).toBe("New Name");

    await act(async () => {
      await result.current.save();
    });

    expect(updateMe).toHaveBeenCalledTimes(1);
    expect(updateMe).toHaveBeenCalledWith(
      expect.objectContaining({
        full_name: "New Name",
        headline: "Backend Engineer",
        phone: "+62 812",
        portfolio_url: "https://jane.dev",
        linkedin_url: "https://linkedin.com/in/jane",
        github_url: "https://github.com/jane",
      }),
      "test-token"
    );
    expect(result.current.status).toBe("saved");
  });

  it("Test useMe.save keeps a failed submission", async () => {
    vi.mocked(updateMe).mockRejectedValue(new Error("network"));
    const { result } = renderHook(() => useMe());

    await waitFor(() => {
      expect(result.current.user?.id).toBe("user-1");
    });

    act(() => {
      result.current.setField("full_name", "Changed Locally");
    });
    expect(result.current.form.full_name).toBe("Changed Locally");

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.error?.message).toBe("network");
    expect(result.current.status).toBe("idle");
    expect(result.current.form.full_name).toBe("Changed Locally");
  });

  it("Test useMe.save clears a prior saved status when a later save fails", async () => {
    vi.mocked(updateMe)
      .mockResolvedValueOnce(sampleUser)
      .mockRejectedValueOnce(new Error("second save failed"));
    const { result } = renderHook(() => useMe());

    await waitFor(() => {
      expect(result.current.user?.id).toBe("user-1");
    });

    act(() => {
      result.current.setField("full_name", "First Edit");
    });
    let firstResult: boolean | undefined;
    await act(async () => {
      firstResult = await result.current.save();
    });
    expect(firstResult).toBe(true);
    expect(result.current.status).toBe("saved");

    act(() => {
      result.current.setField("full_name", "Second Edit");
    });
    let secondResult: boolean | undefined;
    await act(async () => {
      secondResult = await result.current.save();
    });
    expect(secondResult).toBe(false);
    expect(result.current.status).toBe("idle");
    expect(result.current.error?.message).toBe("second save failed");
  });

  it("Test useMe.deactivate signs out", async () => {
    const { result } = renderHook(() => useMe());

    await waitFor(() => {
      expect(result.current.user?.id).toBe("user-1");
    });

    await act(async () => {
      await result.current.deactivate();
    });

    expect(deactivateMe).toHaveBeenCalledTimes(1);
    expect(signOut).toHaveBeenCalledTimes(1);
  });
});
