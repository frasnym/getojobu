"use client";

import React from "react";
import { Download } from "lucide-react";

interface DownloadCoverLetterButtonProps {
  text: string;
  fileName: string;
  className?: string;
}

const PRINT_ROOT_ID = "cover-letter-print-root";

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
      @media print {
        body > *:not(#${PRINT_ROOT_ID}) { visibility: hidden !important; }
        #${PRINT_ROOT_ID} {
          visibility: visible !important;
          position: absolute !important;
          left: 0 !important;
          top: 0 !important;
          width: 100% !important;
          margin: 0 !important;
          padding: 0 !important;
        }
        #${PRINT_ROOT_ID} .cover-letter-print-text {
          visibility: visible !important;
          white-space: pre-wrap !important;
          font-family: Georgia, "Times New Roman", serif !important;
          font-size: 12pt !important;
          line-height: 1.6 !important;
          color: #000 !important;
        }
        @page { margin: 2cm; }
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

    try {
      window.print();
    } finally {
      document.title = previousTitle;
      root.remove();
      style.remove();
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
