import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
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

const readyContent = {
  text: "Dear Hiring Manager, I am excited to apply for the Senior Go Engineer role.",
  metadata: { model: "gpt-4o", prompt_version: "cover-letter-v1" },
};

const fileName = "Cover Letter_Jane Doe_Software Engineer.pdf";

describe("CoverLetterTab component", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the generated letter text and a download button when ready", () => {
    render(
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        content={readyContent}
        fileName={fileName}
      />
    );

    expect(screen.getByText(readyContent.text)).toBeInTheDocument();
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
        content={readyContent}
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

  it("shows the produced variant badge and selects the variant that generated the letter", () => {
    render(
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        content={{
          text: readyContent.text,
          metadata: { model: "gpt-4o", prompt_version: "generate-cover-letter-full-v1" },
        }}
        fileName={fileName}
      />
    );

    expect(screen.getByText(/Generated with full/i)).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Full/i })).toBeChecked();
    expect(screen.getByRole("radio", { name: /Short/i })).not.toBeChecked();
  });

  it("syncs the selector when refreshed content was generated with a different variant", () => {
    const renderWithVariant = (variant: "short" | "full") => (
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        content={{
          text: readyContent.text,
          metadata: { model: "gpt-4o", prompt_version: `generate-cover-letter-${variant}-v1` },
        }}
        fileName={fileName}
      />
    );

    const { rerender } = render(renderWithVariant("full"));
    expect(screen.getByRole("radio", { name: /Full/i })).toBeChecked();

    rerender(renderWithVariant("short"));
    expect(screen.getByRole("radio", { name: /Short/i })).toBeChecked();
    expect(screen.getByText(/Generated with short/i)).toBeInTheDocument();
  });

  it("keeps the print DOM until afterprint, then cleans up the letter and style", () => {
    const printSpy = vi.spyOn(window, "print").mockImplementation(() => {
      expect(document.title).toBe(fileName);
      expect(document.getElementById("cover-letter-print-root")).not.toBeNull();
    });

    render(
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        content={readyContent}
        fileName={fileName}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Download/i }));

    expect(printSpy).toHaveBeenCalledTimes(1);
    // window.print() returns before the preview renders: cleanup must be
    // deferred, otherwise the PDF is blank.
    expect(document.title).toBe(fileName);
    expect(document.getElementById("cover-letter-print-root")).not.toBeNull();
    expect(document.querySelector("style[data-cover-letter-print]")).not.toBeNull();

    document.dispatchEvent(new Event("afterprint"));

    expect(document.title).not.toBe(fileName);
    expect(document.getElementById("cover-letter-print-root")).toBeNull();
    expect(document.querySelector("style[data-cover-letter-print]")).toBeNull();
    printSpy.mockRestore();
  });

  it("falls back to a timeout when afterprint never fires", () => {
    vi.useFakeTimers();
    const printSpy = vi.spyOn(window, "print").mockImplementation(() => {});

    render(
      <CoverLetterTab
        evalId="eval-001"
        status="success"
        content={readyContent}
        fileName={fileName}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Download/i }));
    expect(document.getElementById("cover-letter-print-root")).not.toBeNull();

    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(document.getElementById("cover-letter-print-root")).toBeNull();
    expect(document.title).not.toBe(fileName);
    printSpy.mockRestore();
    vi.useRealTimers();
  });
});
