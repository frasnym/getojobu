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
});
