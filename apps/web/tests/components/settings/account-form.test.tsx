import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import React from "react";
import { validateAccountForm } from "@/lib/account-validation";
import { AccountForm } from "@/components/settings/account-form";
import { AccountUser, UpdateAccountInput } from "@/lib/api/types";

const { setField, save, deactivate, refetch, useMe } = vi.hoisted(() => ({
  setField: vi.fn(),
  save: vi.fn().mockResolvedValue(true),
  deactivate: vi.fn().mockResolvedValue(undefined),
  refetch: vi.fn(),
  useMe: vi.fn(),
}));

vi.mock("@/hooks/use-me", () => ({ useMe }));

const validForm: UpdateAccountInput = {
  full_name: "Jane Doe",
  headline: "Backend Engineer",
  phone: "+62 812",
  portfolio_url: "https://jane.dev",
  linkedin_url: "https://linkedin.com/in/jane",
  github_url: "https://github.com/jane",
};

const baseUser: AccountUser = {
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

type Overrides = Partial<{
  user: AccountUser | null;
  form: UpdateAccountInput;
  loading: boolean;
  saving: boolean;
  error: Error | null;
  status: "idle" | "saved";
}>;

function renderForm(overrides: Overrides = {}) {
  useMe.mockReturnValue({
    user: baseUser,
    form: { ...validForm },
    setField,
    loading: false,
    saving: false,
    error: null,
    status: "idle",
    save,
    deactivate,
    refetch,
    ...overrides,
  });
  render(<AccountForm />);
}

function openDeactivateDialog() {
  fireEvent.click(screen.getByRole("button", { name: "Deactivate account" }));
  return screen.getByRole("dialog", { name: /deactivate account/i });
}

describe("validateAccountForm", () => {
  it("returns no errors for a valid form", () => {
    expect(validateAccountForm(validForm)).toEqual({});
  });

  it("returns a keyed error for an invalid portfolio URL", () => {
    const errors = validateAccountForm({ ...validForm, portfolio_url: "not-a-url" });
    expect(errors).toHaveProperty("portfolio_url");
    expect(errors).not.toHaveProperty("linkedin_url");
  });

  it("returns a keyed error for a whitespace-only full name", () => {
    const errors = validateAccountForm({ ...validForm, full_name: "   " });
    expect(errors).toHaveProperty("full_name");
  });

  it("accepts null values", () => {
    const empty: UpdateAccountInput = {
      full_name: null,
      headline: null,
      phone: null,
      portfolio_url: null,
      linkedin_url: null,
      github_url: null,
    };
    expect(validateAccountForm(empty)).toEqual({});
  });

  it("requires an absolute http/https URL with a host", () => {
    expect(validateAccountForm({ ...validForm, github_url: "http:foo" })).toHaveProperty(
      "github_url"
    );
    expect(validateAccountForm({ ...validForm, github_url: "https://" })).toHaveProperty(
      "github_url"
    );
    expect(validateAccountForm({ ...validForm, github_url: "ftp://example.com" })).toHaveProperty(
      "github_url"
    );
  });

  it("enforces the backend length limits", () => {
    expect(validateAccountForm({ ...validForm, full_name: "a".repeat(129) })).toHaveProperty(
      "full_name"
    );
    expect(validateAccountForm({ ...validForm, headline: "a".repeat(257) })).toHaveProperty(
      "headline"
    );
    expect(validateAccountForm({ ...validForm, phone: "a".repeat(65) })).toHaveProperty("phone");
    expect(validateAccountForm({ ...validForm, github_url: "https://x.dev/" + "a".repeat(2040) })).toHaveProperty(
      "github_url"
    );
  });
});

describe("AccountForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    save.mockResolvedValue(true);
    deactivate.mockResolvedValue(undefined);
  });

  it("renders the six editable inputs plus read-only Clerk email", () => {
    renderForm();
    expect(screen.getByRole("heading", { level: 1, name: "Account" })).toBeInTheDocument();
    expect(screen.getByText("Account details")).toBeInTheDocument();
    expect(screen.getByText("Account state")).toBeInTheDocument();

    for (const label of [
      "Full name",
      "Headline",
      "Phone",
      "Portfolio URL",
      "LinkedIn URL",
      "GitHub URL",
    ]) {
      expect(screen.getByLabelText(label)).toBeInTheDocument();
    }

    const email = screen.getByLabelText("Email");
    expect(email).toHaveAttribute("readonly");
    expect(email).toHaveValue("jane@example.com");
    expect(screen.getByText("Managed by Clerk")).toBeInTheDocument();
    expect(screen.getByText(/Member since/i)).toBeInTheDocument();
  });

  it("shows a field error and skips save for an invalid URL", () => {
    renderForm({ form: { ...validForm, portfolio_url: "not-a-url" } });
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(screen.getByText(/must be an http or https URL with a host/i)).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });

  it("shows a status message when the save succeeded", () => {
    renderForm({ status: "saved" });
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("hides the saved message after an edit until the next save", async () => {
    renderForm({ status: "saved" });
    expect(screen.getByRole("status")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Full name"), { target: { value: "New Name" } });
    expect(setField).toHaveBeenCalledWith("full_name", "New Name");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    await waitFor(() => expect(screen.getByRole("status")).toBeInTheDocument());
  });

  it("does not show the success banner when a second save fails", async () => {
    save.mockResolvedValueOnce(false);
    renderForm({ status: "saved" });
    expect(screen.getByRole("status")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Full name"), { target: { value: "New Name" } });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("disables Save while the account is still loading", () => {
    renderForm({ loading: true });
    expect(screen.getByRole("button", { name: /save changes/i })).toBeDisabled();
  });

  it("enables Save once loading finishes", () => {
    renderForm({ loading: false, saving: false });
    expect(screen.getByRole("button", { name: /save changes/i })).toBeEnabled();
  });

  it("shows an API error while keeping entered values", () => {
    renderForm({ error: new Error("save failed") });
    expect(screen.getByRole("alert")).toHaveTextContent("save failed");
    expect(screen.getByLabelText("Full name")).toHaveValue("Jane Doe");
  });

  it("only deactivates on Confirm, not Cancel, and focuses Cancel on open", async () => {
    renderForm();
    const dialog = openDeactivateDialog();
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Cancel" }));

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(deactivate).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    openDeactivateDialog();
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));
    await waitFor(() => expect(deactivate).toHaveBeenCalledTimes(1));
  });

  it("shows an error when deactivation rejects instead of leaving an unhandled rejection", async () => {
    deactivate.mockRejectedValueOnce(new Error("network down"));
    renderForm();
    openDeactivateDialog();
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("network down"));
    expect(deactivate).toHaveBeenCalledTimes(1);
  });
});
