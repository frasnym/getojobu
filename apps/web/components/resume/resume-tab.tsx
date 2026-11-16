import React, { useState } from "react";
import { DownloadResumeButton } from "./download-resume-button";
import { retryResumeGeneration } from "@/lib/api/endpoints";
import { FileText, CheckCircle2, AlertCircle, Clock, Sparkles, Loader2, RotateCw } from "lucide-react";

interface ResumeTabProps {
  evalId?: string;
  resumeStatus?: string;
  cvImprovements?: string[];
  token?: string | null;
  /** Called after a retry request is queued successfully, so the parent can refetch the latest status. */
  onRetried?: () => void;
}

export function ResumeTab({
  evalId,
  resumeStatus,
  cvImprovements = [],
  token,
  onRetried,
}: ResumeTabProps) {
  const [retrying, setRetrying] = useState(false);
  const [retryError, setRetryError] = useState<string | null>(null);

  const handleRetry = async () => {
    if (!evalId || retrying) return;
    setRetrying(true);
    setRetryError(null);
    try {
      await retryResumeGeneration(evalId, token);
      onRetried?.();
    } catch (err: any) {
      setRetryError(err?.message || "Failed to queue resume generation. Please try again.");
    } finally {
      setRetrying(false);
    }
  };
  if (!evalId || !resumeStatus) {
    return (
      <div className="rounded-2xl border border-dashed border-border/80 bg-card/40 p-12 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary mb-4">
          <FileText className="h-6 w-6 text-muted-foreground" />
        </div>
        <h3 className="text-lg font-bold text-foreground">No Tailored Resume Generated</h3>
        <p className="mt-2 text-sm text-muted-foreground max-w-md mx-auto">
          Tailored resumes are generated in tandem with your compatibility evaluation. Run an analysis on this job to generate a custom PDF.
        </p>
      </div>
    );
  }

  const isReady = resumeStatus === "ready" || resumeStatus === "success";
  const isGenerating = resumeStatus === "generating" || resumeStatus === "pending";
  const isFailed = resumeStatus === "failed";

  return (
    <div className="space-y-6">
      {/* Top Banner & Download Section */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-6 rounded-xl border border-border/60 bg-card p-6">
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
                Generating Resume...
              </span>
            )}
            {isFailed && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-destructive/10 px-2.5 py-0.5 text-xs font-bold text-destructive">
                <AlertCircle className="h-3.5 w-3.5" />
                Generation Failed
              </span>
            )}
          </div>

          <h3 className="text-lg font-bold text-foreground">Customized Application Resume</h3>
          <p className="text-xs text-muted-foreground max-w-lg">
            This resume has been compiled and styled specifically for this listing, highlighting your strongest relevant experiences and matching skills.
          </p>
        </div>

        {/* Action Button */}
        {isReady && (
          <div className="shrink-0">
            <DownloadResumeButton evalId={evalId} token={token} />
          </div>
        )}
        {isFailed && (
          <div className="shrink-0 flex flex-col items-start gap-2">
            <button
              type="button"
              onClick={handleRetry}
              disabled={retrying}
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {retrying ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RotateCw className="h-4 w-4" />
              )}
              <span>{retrying ? "Retrying..." : "Retry Generation"}</span>
            </button>
            {retryError && (
              <p className="flex items-center gap-1.5 text-xs text-destructive font-medium">
                <AlertCircle className="h-3.5 w-3.5" />
                <span>{retryError}</span>
              </p>
            )}
          </div>
        )}
      </div>

      {/* Tailored CV Improvement Suggestions */}
      {cvImprovements && cvImprovements.length > 0 && (
        <div className="rounded-xl border border-border/60 bg-card p-6 space-y-4">
          <div className="flex items-center gap-2">
            <Sparkles className="h-4 w-4 text-emerald-500" />
            <h4 className="text-sm font-bold text-foreground">
              Key Tailoring Adjustments in this Version
            </h4>
          </div>
          <p className="text-xs text-muted-foreground">
            The AI tailored your resume bullets and structure based on the employer's specific requirements:
          </p>

          <ul className="space-y-2.5 text-xs text-foreground/90">
            {cvImprovements.map((tip, idx) => (
              <li
                key={idx}
                className="flex items-start gap-2.5 rounded-lg border border-border/40 bg-background/50 p-3"
              >
                <span className="font-bold text-emerald-500">#{idx + 1}</span>
                <span className="leading-relaxed">{tip}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Additional application advice */}
      <div className="rounded-xl border border-border/40 bg-card/40 p-5 text-xs text-muted-foreground space-y-1">
        <span className="font-semibold text-foreground">Notice Before Applying:</span>
        <p>
          Always review the downloaded PDF before submitting to verify personal contact information and employment dates.
        </p>
      </div>
    </div>
  );
}
