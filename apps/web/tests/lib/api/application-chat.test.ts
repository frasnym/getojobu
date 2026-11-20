import { afterEach, describe, expect, it, vi } from "vitest";
import { answerApplicationQuestion } from "@/lib/api/endpoints";

const originalFetch = global.fetch;

afterEach(() => {
  global.fetch = originalFetch;
});

function streamResponse(frames: string[]): Response {
  const encoder = new TextEncoder();
  return new Response(
    new ReadableStream({
      start(controller) {
        for (const frame of frames) controller.enqueue(encoder.encode(frame));
        controller.close();
      },
    }),
    { status: 200, headers: { "Content-Type": "text/event-stream" } }
  );
}

describe("answerApplicationQuestion", () => {
  it("streams deltas and stops at the done frame", async () => {
    const fetchMock = vi.fn(async () =>
      streamResponse([
        'data: {"delta":"Five years"}\n\n',
        'data: {"delta":" of Go."}\n\ndata: {"done":true}\n\n',
      ])
    );
    global.fetch = fetchMock as any;

    const deltas: string[] = [];
    await answerApplicationQuestion("job-1", "How many years?", [], "test-token", (delta) =>
      deltas.push(delta)
    );

    expect(deltas).toEqual(["Five years", " of Go."]);
    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/v1/jobs/job-1/chat");
    expect((options.headers as Record<string, string>).Authorization).toBe("Bearer test-token");
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
      answerApplicationQuestion("job-1", "", [], null, () => {})
    ).rejects.toThrow("question is required");
  });
});
