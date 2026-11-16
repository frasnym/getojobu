import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { PaginationControls } from "@/components/jobs/pagination-controls";

describe("PaginationControls component", () => {
  it("renders range, total, and page indicator properly", () => {
    render(
      <PaginationControls
        page={1}
        limit={10}
        totalItems={45}
        totalPages={5}
        hasNext={true}
        hasPrev={false}
        onPageChange={vi.fn()}
        onLimitChange={vi.fn()}
      />
    );

    expect(screen.getByText(/Showing/i)).toBeInTheDocument();
    expect(screen.getByText("45")).toBeInTheDocument();
    expect(screen.getAllByText("1")).toHaveLength(2);
    expect(screen.getAllByText("10")).toHaveLength(2);
    expect(screen.getByText(/Page/)).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("handles limit change options [10, 20, 50]", () => {
    const onLimitChange = vi.fn();
    render(
      <PaginationControls
        page={1}
        limit={10}
        totalItems={50}
        totalPages={5}
        hasNext={true}
        hasPrev={false}
        onPageChange={vi.fn()}
        onLimitChange={onLimitChange}
        limitOptions={[10, 20, 50]}
      />
    );

    const btn20 = screen.getByRole("button", { name: "20" });
    const btn50 = screen.getByRole("button", { name: "50" });

    expect(btn20).toBeInTheDocument();
    expect(btn50).toBeInTheDocument();

    fireEvent.click(btn20);
    expect(onLimitChange).toHaveBeenCalledWith(20);

    fireEvent.click(btn50);
    expect(onLimitChange).toHaveBeenCalledWith(50);
  });

  it("disables prev button on page 1 and triggers onPageChange on next click", () => {
    const onPageChange = vi.fn();
    render(
      <PaginationControls
        page={1}
        limit={10}
        totalItems={30}
        totalPages={3}
        hasNext={true}
        hasPrev={false}
        onPageChange={onPageChange}
        onLimitChange={vi.fn()}
      />
    );

    const prevBtn = screen.getByRole("button", { name: /previous/i });
    const nextBtn = screen.getByRole("button", { name: /next/i });

    expect(prevBtn).toBeDisabled();
    expect(nextBtn).not.toBeDisabled();

    fireEvent.click(nextBtn);
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it("disables next button on last page and triggers onPageChange on prev click", () => {
    const onPageChange = vi.fn();
    render(
      <PaginationControls
        page={3}
        limit={10}
        totalItems={30}
        totalPages={3}
        hasNext={false}
        hasPrev={true}
        onPageChange={onPageChange}
        onLimitChange={vi.fn()}
      />
    );

    const prevBtn = screen.getByRole("button", { name: /previous/i });
    const nextBtn = screen.getByRole("button", { name: /next/i });

    expect(prevBtn).not.toBeDisabled();
    expect(nextBtn).toBeDisabled();

    fireEvent.click(prevBtn);
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it("returns null when totalItems is 0", () => {
    const { container } = render(
      <PaginationControls
        page={1}
        limit={10}
        totalItems={0}
        totalPages={0}
        hasNext={false}
        hasPrev={false}
        onPageChange={vi.fn()}
        onLimitChange={vi.fn()}
      />
    );

    expect(container.firstChild).toBeNull();
  });
});
