import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import React from "react";
import { CoverLetterTab } from "@/components/cover-letter/cover-letter-tab";
import * as api from "@/lib/api/endpoints";

vi.mock("@/lib/api/endpoints", () => ({
  generateCoverLetter: vi.fn().mockResolvedValue({
    id: "eval-001",
    user_job_id: "uj-001",
    evaluation_status: "queued",
    cover_letter_status: "queued",
  }),
}));

const readyLetter = "Dear Hiring Manager, I am excited to apply for the Senior Go Engineer role.";

const fileName = "Jane Doe_Cover Letter_Software Engineer.pdf";

describe("CoverLetterTab component", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the generated letter text and a download button when ready", () => {
    render(
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        text={readyLetter}
        fileName={fileName}
      />
    );

    expect(screen.getByText(readyLetter)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Download/i })).toBeInTheDocument();
  });

  it("renders the fixed failure text and a Retry button when generation failed", () => {
    render(
      <CoverLetterTab
        evalId="eval-001"
        status="failed"
        failureMessage={`2026-11-17T10:00:00Z — Cover letter generation failed`}
      />
    );

    expect(screen.getByText("Cover letter generation failed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Retry Generation/i })).toBeInTheDocument();
  });

  it("queues a full-variant generation when Full is selected and Generate is clicked", async () => {
    const onQueued = vi.fn();
    render(
      <CoverLetterTab
        evalId="eval-001"
        status={undefined}
        reportStatus="success"
        onQueued={onQueued}
      />
    );

    fireEvent.click(screen.getByRole("radio", { name: /Full/i }));
    fireEvent.click(screen.getByRole("button", { name: /Generate Cover Letter/i }));

    await waitFor(() => {
      expect(api.generateCoverLetter).toHaveBeenCalledWith("eval-001", "full", false);
      expect(onQueued).toHaveBeenCalledTimes(1);
    });
  });

  it("queues a forced short-variant regeneration from the ready state", async () => {
    const onQueued = vi.fn();
    render(
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        text={readyLetter}
        fileName={fileName}
        onQueued={onQueued}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Regenerate Cover Letter/i }));

    await waitFor(() => {
      expect(api.generateCoverLetter).toHaveBeenCalledWith("eval-001", "short", true);
      expect(onQueued).toHaveBeenCalledTimes(1);
    });
  });

  it("hides Generate when the report is not successful", () => {
    render(
      <CoverLetterTab
        evalId="eval-001"
        status={undefined}
        reportStatus="pending"
      />
    );

    expect(
      screen.queryByRole("button", { name: /Generate Cover Letter/i })
    ).not.toBeInTheDocument();
  });

  it("copies the cover letter text and shows feedback when Copy is clicked", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        text={readyLetter}
        fileName={fileName}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Copy cover letter/i }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(readyLetter);
      expect(screen.getByRole("button", { name: /Cover letter copied/i })).toBeInTheDocument();
    });
  });

});
