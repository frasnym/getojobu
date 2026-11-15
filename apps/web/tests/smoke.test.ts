import { describe, it, expect } from "vitest";

describe("Web MVP test environment", () => {
  it("runs in jsdom with testing-library setup", () => {
    expect(true).toBe(true);
    expect(document).toBeDefined();
  });
});
