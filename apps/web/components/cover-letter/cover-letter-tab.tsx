"use client";

import React, { useState } from "react";
import { DownloadCoverLetterButton } from "./download-cover-letter-button";
import { generateCoverLetter } from "@/lib/api/endpoints";
import {
  FileText,
  CheckCircle2,
  AlertCircle,
  Clock,
  Sparkles,
  Loader2,
  RotateCw,
} from "lucide-react";

type Variant = "short" | "full";

interface CoverLetterTabProps {
  evalId?: string;
  status?: string;
  text?: string;
  /**
   * Backend failure history. Intentionally unused by the UI: raw worker
   * exceptions can contain prompt text or CV contents, so only the fixed
   * COVER_LETTER_FAILURE_TEXT is ever rendered. Kept on the contract so the
   * detail page can pass it through without leaking it to the DOM.
   */
  failureMessage?: string;
  fileName?: string;
  /** Cover-letter generation is only offered once the report is successful. */
  reportStatus?: string;
  /** Called after a generate/regenerate request is queued successfully. */
  onQueued?: () => void;
}

// The backend stores an append-only, timestamped history. Only this fixed text
// is ever shown to the user, so raw exceptions can never leak into the UI.
const COVER_LETTER_FAILURE_TEXT = "Cover letter generation failed";

function VariantSelector({
  value,
  onChange,
}: {
  value: Variant;
  onChange: (variant: Variant) => void;
}) {
  return (
    <fieldset className="flex items-center justify-center gap-4">
      <legend className="sr-only">Cover letter length</legend>
      {(["short", "full"] as const).map((option) => (
        <label
          key={option}
          className="inline-flex cursor-pointer items-center gap-2 text-sm font-semibold text-foreground"
        >
          <input
            type="radio"
            name="cover-letter-variant"
            value={option}
            aria-label={option === "short" ? "Short" : "Full"}
            checked={value === option}
            onChange={() => onChange(option)}
          />
          <span>{option === "short" ? "Short" : "Full"}</span>
        </label>
      ))}
    </fieldset>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="rounded-2xl border border-dashed border-border/80 bg-card/40 p-12 text-center">
      <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
        <FileText className="h-6 w-6 text-muted-foreground" />
      </div>
      <h3 className="text-lg font-bold text-foreground">No Cover Letter Generated</h3>
      <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">{message}</p>
    </div>
  );
}

export function CoverLetterTab({
  evalId,
  status,
  text,
  fileName,
  reportStatus,
  onQueued,
}: CoverLetterTabProps) {
  const [variant, setVariant] = useState<Variant>("short");
  const [submitting, setSubmitting] = useState(false);
  const [requestError, setRequestError] = useState<string | null>(null);

  const handleGenerate = async (force: boolean) => {
    if (!evalId || submitting) return;
    setSubmitting(true);
    setRequestError(null);
    try {
      await generateCoverLetter(evalId, variant, force);
      onQueued?.();
    } catch (err) {
      setRequestError(
        err instanceof Error
          ? err.message
          : "Failed to queue cover letter generation. Please try again."
      );
    } finally {
      setSubmitting(false);
    }
  };

  const hasEval = Boolean(evalId);
  const notStarted = !status;
  const canGenerate = hasEval && notStarted && reportStatus === "success";
  const isReady = status === "success" || status === "ready";
  const isGenerating =
    status === "queued" ||
    status === "queued_retry" ||
    status === "pending" ||
    status === "generating";
  const isFailed = status === "failed";

  // No evaluation exists yet — there is nothing to build a cover letter from.
  if (!hasEval) {
    return (
      <EmptyState message="Run an analysis on this job first, then generate a tailored cover letter from the successful match report." />
    );
  }

  // Evaluation exists but no letter has been requested yet. Generation is only
  // offered when the match report succeeded; otherwise the Generate button is
  // hidden entirely.
  if (notStarted) {
    if (!canGenerate) {
      return (
        <EmptyState message="Cover letters are generated once the match report for this job is complete." />
      );
    }

    return (
      <div className="rounded-2xl border border-dashed border-border/80 bg-card/40 p-12 text-center space-y-4">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
          <Sparkles className="h-6 w-6 text-muted-foreground" />
        </div>
        <h3 className="text-lg font-bold text-foreground">Generate Your Cover Letter</h3>
        <p className="mx-auto max-w-md text-sm text-muted-foreground">
          Your match report is ready. Choose a length and generate a tailored cover letter.
        </p>
        <VariantSelector value={variant} onChange={setVariant} />
        <div>
          <button
            type="button"
            onClick={() => handleGenerate(false)}
            disabled={submitting}
            className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors disabled:opacity-50"
          >
            {submitting ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Sparkles className="h-4 w-4" />
            )}
            <span>{submitting ? "Queuing..." : "Generate Cover Letter"}</span>
          </button>
        </div>
        {requestError && (
          <p className="flex items-center justify-center gap-1.5 text-xs text-destructive font-medium">
            <AlertCircle className="h-3.5 w-3.5" />
            <span>{requestError}</span>
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Status banner and actions */}
      <div className="flex flex-col gap-6 rounded-xl border border-border/60 bg-card p-6 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            {isReady && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-xs font-bold text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="h-3.5 w-3.5" />
                Ready for Application
              </span>
            )}
            {isGenerating && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-500/10 px-2.5 py-0.5 text-xs font-bold text-blue-600 dark:text-blue-400">
                <Clock className="h-3.5 w-3.5" />
                Generating Cover Letter...
              </span>
            )}
            {isFailed && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-red-500/10 px-2.5 py-0.5 text-xs font-bold text-red-400 dark:text-red-300">
                <AlertCircle className="h-3.5 w-3.5" />
                Generation Failed
              </span>
            )}
          </div>

          <h3 className="text-lg font-bold text-foreground">Tailored Cover Letter</h3>
          <p className="max-w-lg text-xs text-muted-foreground">
            A letter tailored to this listing. Review it before downloading and applying.
          </p>

          {isFailed && (
            <p className="flex items-center gap-1.5 text-xs font-medium text-destructive">
              <AlertCircle className="h-3.5 w-3.5" />
              <span>{COVER_LETTER_FAILURE_TEXT}</span>
            </p>
          )}
          {requestError && (
            <p className="flex items-center gap-1.5 text-xs font-medium text-destructive">
              <AlertCircle className="h-3.5 w-3.5" />
              <span>{requestError}</span>
            </p>
          )}
        </div>

        <div className="flex shrink-0 flex-col items-start gap-3">
          {(isReady || isFailed) && (
            <VariantSelector value={variant} onChange={setVariant} />
          )}
          {isReady && (
            <DownloadCoverLetterButton text={text ?? ""} fileName={fileName ?? "Cover Letter.pdf"} />
          )}
          {isFailed && (
            <button
              type="button"
              onClick={() => handleGenerate(true)}
              disabled={submitting}
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {submitting ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RotateCw className="h-4 w-4" />
              )}
              <span>{submitting ? "Retrying..." : "Retry Generation"}</span>
            </button>
          )}
          {isReady && (
            <button
              type="button"
              onClick={() => handleGenerate(true)}
              disabled={submitting}
              className="inline-flex items-center gap-2 rounded-lg border border-border/60 px-4 py-2 text-xs font-semibold text-foreground transition-colors hover:bg-secondary disabled:opacity-50"
            >
              <Sparkles className="h-3.5 w-3.5" />
              <span>Regenerate Cover Letter</span>
            </button>
          )}
          {isGenerating && (
            <p className="text-xs text-muted-foreground">
              This runs in the background. Use Refresh to check the latest status.
            </p>
          )}
        </div>
      </div>

      {/* The letter itself. A failed regeneration keeps the previous letter visible. */}
      {text ? (
        <div className="whitespace-pre-wrap rounded-xl border border-border/60 bg-card p-6 text-sm leading-relaxed text-foreground/90">
          {text}
        </div>
      ) : (
        <div className="rounded-xl border border-border/40 bg-card/40 p-6 text-sm text-muted-foreground">
          {isGenerating
            ? "Your cover letter is being generated in the background."
            : "No letter text is available for this attempt."}
        </div>
      )}

      <div className="space-y-1 rounded-xl border border-border/40 bg-card/40 p-5 text-xs text-muted-foreground">
        <span className="font-semibold text-foreground">Before Applying:</span>
        <p>Always review the generated letter for accuracy before submitting it to an employer.</p>
      </div>
    </div>
  );
}
