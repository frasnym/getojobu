import { UpdateAccountInput } from "@/lib/api/types";

const URL_FIELDS = ["portfolio_url", "linkedin_url", "github_url"] as const;

function runeLength(value: string): number {
  return Array.from(value).length;
}

function isHttpUrl(value: string): boolean {
  if (!/^https?:\/\//.test(value)) return false;
  try {
    const parsed = new URL(value);
    return (
      (parsed.protocol === "http:" || parsed.protocol === "https:") &&
      parsed.hostname !== ""
    );
  } catch {
    return false;
  }
}

// Mirrors the backend's decodeUpdateMeRequest rules so users see errors before
// a request is sent. Keys are the JSON field names sent to PATCH /v1/me.
export function validateAccountForm(
  form: UpdateAccountInput
): Record<string, string> {
  const errors: Record<string, string> = {};

  if (form.full_name !== null) {
    const value = form.full_name.trim();
    if (value === "") {
      errors.full_name = "must not be blank";
    } else if (runeLength(value) > 128) {
      errors.full_name = "is too long";
    }
  }

  if (form.headline !== null) {
    const value = form.headline.trim();
    if (value === "") {
      errors.headline = "must not be blank";
    } else if (runeLength(value) > 256) {
      errors.headline = "is too long";
    }
  }

  if (form.phone !== null && runeLength(form.phone.trim()) > 64) {
    errors.phone = "is too long";
  }

  for (const field of URL_FIELDS) {
    const raw = form[field];
    if (raw === null) continue;
    const value = raw.trim();
    if (runeLength(value) > 2048) {
      errors[field] = "is too long";
    } else if (!isHttpUrl(value)) {
      errors[field] = "must be an http or https URL with a host";
    }
  }

  return errors;
}
