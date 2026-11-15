import React, { useState } from "react";
import { Download, RefreshCw, AlertCircle } from "lucide-react";
import { getResumeDownloadUrl } from "@/lib/api/endpoints";

interface DownloadResumeButtonProps {
  evalId: string;
  token?: string | null;
  className?: string;
}

export function DownloadResumeButton({
  evalId,
  token,
  className = "",
}: DownloadResumeButtonProps) {
  const [downloading, setDownloading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleDownload = async () => {
    try {
      setDownloading(true);
      setError(null);
      const res = await getResumeDownloadUrl(evalId, token);
      if (res && res.download_url) {
        // Trigger download via anchor element to avoid browser popup blocker restrictions on async window.open
        const link = document.createElement("a");
        link.href = res.download_url;
        link.download = res.filename || "tailored_resume.pdf";
        link.target = "_blank";
        link.rel = "noopener noreferrer";
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
      } else {
        throw new Error("Presigned download URL not returned by server");
      }
    } catch (err: any) {
      console.error("Failed to download tailored resume:", err);
      setError(err?.message || "Failed to retrieve download link. Please try again.");
    } finally {
      setDownloading(false);
    }
  };

  return (
    <div className="flex flex-col items-start gap-2">
      <button
        type="button"
        onClick={handleDownload}
        disabled={downloading}
        className={`inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors disabled:opacity-50 ${className}`}
      >
        {downloading ? (
          <RefreshCw className="h-4 w-4 animate-spin" />
        ) : (
          <Download className="h-4 w-4" />
        )}
        <span>{downloading ? "Preparing PDF..." : "Download Tailored PDF"}</span>
      </button>

      {error && (
        <p className="flex items-center gap-1.5 text-xs text-destructive font-medium">
          <AlertCircle className="h-3.5 w-3.5" />
          <span>{error}</span>
        </p>
      )}
    </div>
  );
}
