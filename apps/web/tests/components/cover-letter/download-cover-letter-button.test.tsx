import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  buildCoverLetterPdf,
  DownloadCoverLetterButton,
  layoutCoverLetter,
  toPdfSafeText,
} from "@/components/cover-letter/download-cover-letter-button";

const layout = { pageWidth: 210, pageHeight: 297, margin: 20, lineHeight: 6.5 };

describe("cover letter PDF helpers", () => {
  it("turns smart quotes and dashes into ASCII before drawing text", () => {
    expect(toPdfSafeText("\u201CHello\u201D \u2014 it\u2019s\u2026")).toBe(
      '"Hello" - it\'s...'
    );
  });

  it("keeps Latin accents but replaces characters the standard PDF fonts cannot draw", () => {
    expect(toPdfSafeText("caf\u00e9 \ud83d\ude00")).toBe("caf\u00e9 ?");
  });

  it("starts the first line at the page margin instead of the top edge", () => {
    expect(
      layoutCoverLetter("Hello", (line) => [line], layout)
    ).toEqual([{ text: "Hello", x: 20, y: 20, page: 1 }]);
  });

  it("wraps a long line at the usable width and stacks the pieces", () => {
    const splitLine = vi.fn((line: string) =>
      line === "long" ? ["part1", "part2"] : [line]
    );

    expect(layoutCoverLetter("long", splitLine, layout)).toEqual([
      { text: "part1", x: 20, y: 20, page: 1 },
      { text: "part2", x: 20, y: 26.5, page: 1 },
    ]);
    expect(splitLine).toHaveBeenCalledWith("long", 170);
  });

  it("starts a new page before text would cross the bottom margin", () => {
    const smallPage = { pageWidth: 210, pageHeight: 40, margin: 10, lineHeight: 6.5 };

    const lines = layoutCoverLetter(
      "a\nb\nc\nd\ne",
      (line) => [line],
      smallPage
    );

    expect(lines.map((line) => [line.text, line.y, line.page])).toEqual([
      ["a", 10, 1],
      ["b", 16.5, 1],
      ["c", 23, 1],
      ["d", 10, 2],
      ["e", 16.5, 2],
    ]);
  });
});

describe("buildCoverLetterPdf", () => {
  it("produces an A4 PDF containing the sanitized letter text", () => {
    const doc = buildCoverLetterPdf("\u201CDear Hiring Manager\u201D \u2014 hello");

    expect(doc.internal.pageSize.getWidth()).toBeCloseTo(210, 1);
    expect(doc.internal.pageSize.getHeight()).toBeCloseTo(297, 1);
    expect(doc.getNumberOfPages()).toBe(1);
    expect(doc.output()).toContain('"Dear Hiring Manager" - hello');
  });
});

describe("DownloadCoverLetterButton", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("does not open the browser print dialog or inject print DOM", () => {
    const printSpy = vi.spyOn(window, "print").mockImplementation(() => undefined);

    render(
      <DownloadCoverLetterButton
        text={"Dear Hiring Manager,\n\nI am excited to apply."}
        fileName="Jane Doe_Cover Letter.pdf"
      />
    );
    fireEvent.click(screen.getByRole("button", { name: /Download PDF/i }));

    expect(printSpy).not.toHaveBeenCalled();
    expect(document.getElementById("cover-letter-print-root")).toBeNull();
    expect(document.querySelector("style[data-cover-letter-print]")).toBeNull();
  });
});
