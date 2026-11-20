"use client";

import React from "react";
import { Download } from "lucide-react";
import { jsPDF } from "jspdf";

interface DownloadCoverLetterButtonProps {
  text: string;
  fileName: string;
  className?: string;
}

interface PdfLayoutOptions {
  pageWidth: number;
  pageHeight: number;
  margin: number;
  lineHeight: number;
}

interface PositionedLine {
  text: string;
  x: number;
  y: number;
  page: number;
}

const MARGIN_MM = 20;
const FONT_SIZE_PT = 12;
const LINE_HEIGHT_MM = 6.5;

/** The standard PDF fonts cannot draw these characters, so use ASCII equivalents. */
export function toPdfSafeText(text: string): string {
  return text
    .replace(/[\u2018\u2019\u201A\u201B]/g, "'")
    .replace(/[\u201C\u201D\u201E\u201F]/g, '"')
    .replace(/[\u2013\u2014\u2015]/g, "-")
    .replace(/\u2026/g, "...")
    .replace(/\u00A0/g, " ")
    .replace(/[^\x20-\x7E\xA0-\xFF\n]/gu, "?");
}

/** Wrap the letter into lines placed inside the page margins, across pages. */
export function layoutCoverLetter(
  text: string,
  splitLine: (line: string, maxWidth: number) => string[],
  { pageWidth, pageHeight, margin, lineHeight }: PdfLayoutOptions
): PositionedLine[] {
  const contentWidth = pageWidth - margin * 2;
  const lines: PositionedLine[] = [];
  let y = margin;
  let page = 1;

  for (const paragraph of text.split("\n")) {
    const wrapped = paragraph.length ? splitLine(paragraph, contentWidth) : [""];
    for (const line of wrapped) {
      if (y + lineHeight > pageHeight - margin) {
        page += 1;
        y = margin;
      }
      if (line) lines.push({ text: line, x: margin, y, page });
      y += lineHeight;
    }
  }

  return lines;
}

export function buildCoverLetterPdf(text: string): jsPDF {
  const doc = new jsPDF({ unit: "mm", format: "a4" });
  doc.setFont("times", "normal");
  doc.setFontSize(FONT_SIZE_PT);

  const lines = layoutCoverLetter(
    toPdfSafeText(text),
    (line, maxWidth) => doc.splitTextToSize(line, maxWidth),
    {
      pageWidth: doc.internal.pageSize.getWidth(),
      pageHeight: doc.internal.pageSize.getHeight(),
      margin: MARGIN_MM,
      lineHeight: LINE_HEIGHT_MM,
    }
  );

  let currentPage = 1;
  for (const line of lines) {
    if (line.page !== currentPage) {
      doc.addPage();
      currentPage = line.page;
    }
    doc.text(line.text, line.x, line.y);
  }

  return doc;
}

export function DownloadCoverLetterButton({
  text,
  fileName,
  className = "",
}: DownloadCoverLetterButtonProps) {
  const handleDownload = () => {
    buildCoverLetterPdf(text).save(fileName);
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
