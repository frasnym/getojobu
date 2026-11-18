"use client";

import React from "react";
import { Download } from "lucide-react";

interface DownloadCoverLetterButtonProps {
  text: string;
  fileName: string;
  className?: string;
}

const PRINT_ROOT_ID = "cover-letter-print-root";
// Fallback for browsers that do not emit afterprint (e.g. a cancelled dialog).
const PRINT_CLEANUP_FALLBACK_MS = 5000;

/**
 * Creates the PDF on demand through the browser's print dialog. A print-only
 * stylesheet hides the rest of the app and shows just the letter, and the
 * document title becomes the download file name offered by the print dialog.
 * No PDF library, server endpoint, or object storage is involved.
 */
export function DownloadCoverLetterButton({
  text,
  fileName,
  className = "",
}: DownloadCoverLetterButtonProps) {
  const handleDownload = () => {
    const previousTitle = document.title;

    const style = document.createElement("style");
    style.setAttribute("data-cover-letter-print", "true");
    style.textContent = `
      @page { margin: 0; }
      @media print {
        html, body { margin: 0 !important; padding: 0 !important; }
        body > *:not(#${PRINT_ROOT_ID}) { display: none !important; }
        #${PRINT_ROOT_ID} {
          display: block !important;
          width: auto !important;
          margin: 0 !important;
          padding: 2cm !important;
          box-sizing: border-box !important;
        }
        #${PRINT_ROOT_ID} .cover-letter-print-text {
          white-space: pre-wrap !important;
          font-family: Georgia, "Times New Roman", serif !important;
          font-size: 12pt !important;
          line-height: 1.6 !important;
          color: #000 !important;
        }
      }
    `;

    const root = document.createElement("div");
    root.id = PRINT_ROOT_ID;
    const content = document.createElement("div");
    content.className = "cover-letter-print-text";
    content.textContent = text;
    root.appendChild(content);

    document.head.appendChild(style);
    document.body.appendChild(root);
    document.title = fileName || "Cover Letter.pdf";

    let cleanedUp = false;
    let fallbackTimer: number | undefined;
    const cleanup = () => {
      if (cleanedUp) return;
      cleanedUp = true;
      document.removeEventListener("afterprint", cleanup);
      if (fallbackTimer !== undefined) window.clearTimeout(fallbackTimer);
      document.title = previousTitle;
      root.remove();
      style.remove();
    };

    // window.print() resolves before the print preview has rendered. Removing
    // the injected DOM here would produce a blank PDF, so defer cleanup until
    // the dialog closes. The timeout covers browsers that never emit the event.
    document.addEventListener("afterprint", cleanup, { once: true });
    fallbackTimer = window.setTimeout(cleanup, PRINT_CLEANUP_FALLBACK_MS);

    try {
      window.print();
    } catch {
      // Exception safety only: when print() throws, afterprint will not fire,
      // so clean up now instead of leaving injected DOM/styles behind.
      cleanup();
    }
  };

  return (
    <button
      type="button"
      onClick={handleDownload}
      className={`inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors ${className}`}
    >
      <Download className="h-4 w-4" />
      <span>Download PDF</span>
    </button>
  );
}
