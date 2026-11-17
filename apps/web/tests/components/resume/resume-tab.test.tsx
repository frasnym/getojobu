import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import React from "react";
import { ResumeTab } from "@/components/resume/resume-tab";
import * as api from "@/lib/api/endpoints";

vi.mock("@/lib/api/endpoints", () => ({
  getResumeDownloadUrl: vi.fn().mockResolvedValue({
    download_url: "https://storage.getojobu.com/signed-url.pdf",
    expires_at: "2026-11-15T12:00:00Z",
    filename: "Tailored_Resume_Stripe.pdf",
  }),
  retryResumeGeneration: vi.fn().mockResolvedValue({ id: "eval-001", evaluation_status: "pending" }),
}));

describe("ResumeTab component", () => {
  it("renders generation status and tailored CV suggestions", () => {
    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus="ready"
        cvImprovements={[
          "Highlight Go concurrency and channels in previous fintech role",
          "Quantify database query optimization results",
        ]}
      />
    );

    expect(screen.getByText(/Ready for Application/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Highlight Go concurrency and channels in previous fintech role/i)
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Download Tailored PDF/i })
    ).toBeInTheDocument();
  });

  it("triggers presigned URL download when download button is clicked", async () => {
    const appendSpy = vi.spyOn(document.body, "appendChild");

    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus="ready"
        cvImprovements={["Highlight skills"]}
      />
    );

    const downloadBtn = screen.getByRole("button", { name: /Download Tailored PDF/i });
    fireEvent.click(downloadBtn);

    await waitFor(() => {
      expect(api.getResumeDownloadUrl).toHaveBeenCalledWith("eval-001", undefined);
      expect(appendSpy).toHaveBeenCalled();
    });

    appendSpy.mockRestore();
  });

  it("handles unevaluated or missing resume cleanly", () => {
    render(
      <ResumeTab
        evalId={undefined}
        resumeStatus={undefined}
      />
    );

    expect(screen.getByText(/No Tailored Resume Generated/i)).toBeInTheDocument();
  });

  it("shows retry button when generation failed", () => {
    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus="failed"
      />
    );

    expect(screen.getByText(/Generation Failed/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Retry Generation/i })).toBeInTheDocument();
  });

  it("queues retry and notifies parent when retry is clicked", async () => {
    const onRetried = vi.fn();
    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus="failed"
        onRetried={onRetried}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Retry Generation/i }));

    await waitFor(() => {
      expect(api.retryResumeGeneration).toHaveBeenCalledWith("eval-001", undefined);
      expect(onRetried).toHaveBeenCalledTimes(1);
    });
  });

  it("offers a generate button when the report is successful but no resume exists yet", () => {
    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus={undefined}
        reportStatus="success"
      />
    );

    expect(screen.getByText(/Generate Your Tailored Resume/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Generate Tailored Resume/i })
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Retry Generation/i })).not.toBeInTheDocument();
  });

  it("does not offer generation when the report is not yet successful", () => {
    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus={undefined}
        reportStatus="pending"
      />
    );

    expect(
      screen.getByText(/Tailored resumes are generated once the match report/i)
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Generate Tailored Resume/i })).not.toBeInTheDocument();
  });

  it("queues resume generation when the generate button is clicked", async () => {
    const onRetried = vi.fn();
    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus={undefined}
        reportStatus="success"
        onRetried={onRetried}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Generate Tailored Resume/i }));

    await waitFor(() => {
      expect(api.retryResumeGeneration).toHaveBeenCalledWith("eval-001", undefined);
      expect(onRetried).toHaveBeenCalledTimes(1);
    });
  });

  it("shows error message when retry fails", async () => {
    vi.mocked(api.retryResumeGeneration).mockRejectedValueOnce(new Error("not authorized to generate resume for this evaluation"));
    render(
      <ResumeTab
        evalId="eval-001"
        resumeStatus="failed"
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Retry Generation/i }));

    await waitFor(() => {
      expect(screen.getByText(/not authorized to generate resume for this evaluation/i)).toBeInTheDocument();
    });
  });
});
