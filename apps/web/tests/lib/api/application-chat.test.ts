import { afterEach, describe, expect, it, vi } from "vitest";
import { answerApplicationQuestion } from "@/lib/api/endpoints";

const originalFetch = global.fetch;

afterEach(() => {
  global.fetch = originalFetch;
});

describe("answerApplicationQuestion", () => {
  it("returns the answer from the response envelope", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          success: true,
          data: { answer: "Five years of Go." },
          meta: { timestamp: "", requestId: "" },
          error: null,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      )
    );
    global.fetch = fetchMock as any;

    const answer = await answerApplicationQuestion(
      "job-1",
      "How many years?",
      [],
      "test-token"
    );

    expect(answer).toBe("Five years of Go.");
    const [url, options] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/v1/jobs/job-1/chat");
    expect(new Headers(options.headers).get("Authorization")).toBe("Bearer test-token");
    expect(JSON.parse(options.body as string)).toEqual({
      question: "How many years?",
      history: [],
    });
  });

  it("throws when the API returns an error", async () => {
    global.fetch = vi.fn(async () =>
      new Response(JSON.stringify({ error: { message: "question is required" } }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      })
    ) as any;

    await expect(
      answerApplicationQuestion("job-1", "", [], null)
    ).rejects.toThrow("question is required");
  });
});
