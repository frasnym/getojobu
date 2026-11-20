"use client";

import React, { useEffect, useRef, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { AlertCircle, Check, Copy, Loader2, Send, Sparkles } from "lucide-react";
import { answerApplicationQuestion } from "@/lib/api/endpoints";
import { ApplicationChatMessage, Job } from "@/lib/api/types";

interface ApplicationChatTabProps {
	job: Job;
}

export function ApplicationChatTab({ job }: ApplicationChatTabProps) {
	const { getToken } = useAuth();
	const [messages, setMessages] = useState<ApplicationChatMessage[]>([]);
	const [input, setInput] = useState("");
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [copiedIndex, setCopiedIndex] = useState<number | null>(null);
	const bottomRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		bottomRef.current?.scrollIntoView({ behavior: "smooth" });
	}, [messages, loading]);

	const send = async (event: React.FormEvent) => {
		event.preventDefault();
		const question = input.trim();
		if (!question || loading) return;

		const history = messages;
		setInput("");
		setError(null);
		setLoading(true);
		setMessages([...history, { role: "user", content: question }]);

		try {
			const token = await getToken();
			const answer = await answerApplicationQuestion(job.slug, question, history, token);
			setMessages([
				...history,
				{ role: "user", content: question },
				{ role: "assistant", content: answer },
			]);
		} catch (err) {
			setError(err instanceof Error ? err.message : "Could not get an answer");
		} finally {
			setLoading(false);
		}
	};

	const copy = async (text: string, index: number) => {
		try {
			await navigator.clipboard.writeText(text);
			setCopiedIndex(index);
			setTimeout(() => setCopiedIndex(null), 1500);
		} catch {
			setError("Could not copy. Select the text and copy it manually.");
		}
	};

	return (
		<div className="space-y-4">
			<div className="rounded-xl border border-border/60 bg-card p-4">
				<div className="flex items-center gap-2">
					<Sparkles className="h-4 w-4 text-primary" />
					<h2 className="text-sm font-bold text-foreground">Application Helper</h2>
				</div>
				<p className="mt-1 text-xs text-muted-foreground">
					Paste a question from the application form. The answer uses your career profile and this job&apos;s profile, and is one sentence you can copy directly.
				</p>
			</div>

			<div className="min-h-72 space-y-4 rounded-xl border border-border/60 bg-card/40 p-4">
				{messages.length === 0 ? (
					<div className="flex h-56 flex-col items-center justify-center gap-2 text-center">
						<Sparkles className="h-6 w-6 text-muted-foreground" />
						<p className="text-sm font-medium text-foreground">No questions yet</p>
						<p className="max-w-xs text-xs text-muted-foreground">
							Try: “How many years of experience do you have with Go?”
						</p>
					</div>
				) : (
					messages.map((message, index) => (
						<div
							key={index}
							className={`flex ${message.role === "user" ? "justify-end" : "justify-start"}`}
						>
							<div
								className={`group max-w-[85%] rounded-xl px-4 py-2.5 text-sm ${
									message.role === "user"
										? "bg-primary text-primary-foreground"
										: "border border-border/60 bg-card text-foreground"
								}`}
							>
								<span className="whitespace-pre-wrap">{message.content}</span>
								{message.role === "assistant" && message.content && (
									<button
										type="button"
										onClick={() => copy(message.content, index)}
										className="mt-2 inline-flex items-center gap-1 border-t border-border/40 pt-2 text-[11px] font-medium text-muted-foreground hover:text-foreground"
										aria-label="Copy answer"
									>
										{copiedIndex === index ? (
											<Check className="h-3 w-3 text-emerald-500" />
										) : (
											<Copy className="h-3 w-3" />
										)}
										{copiedIndex === index ? "Copied" : "Copy"}
									</button>
								)}
							</div>
						</div>
					))
				)}
				{loading && (
					<div className="flex justify-start">
						<div className="inline-flex items-center gap-2 rounded-xl border border-border/60 bg-card px-4 py-2.5 text-sm text-muted-foreground">
							<Loader2 className="h-3.5 w-3.5 animate-spin" /> Writing answer...
						</div>
					</div>
				)}
				<div ref={bottomRef} />
			</div>

			{error && (
				<div className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
					<AlertCircle className="h-3.5 w-3.5 shrink-0" />
					<span>{error}</span>
				</div>
			)}

			<form onSubmit={send} className="flex items-end gap-2">
				<textarea
					value={input}
					onChange={(event) => setInput(event.target.value)}
					onKeyDown={(event) => {
						if (event.key === "Enter" && !event.shiftKey) {
							event.preventDefault();
							void send(event);
						}
					}}
					rows={2}
					placeholder="Paste the form question here..."
					disabled={loading}
					className="flex-1 resize-none rounded-xl border border-border/60 bg-card px-3 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-60"
					aria-label="Application form question"
				/>
				<button
					type="submit"
					disabled={loading || input.trim() === ""}
					className="inline-flex h-11 items-center gap-2 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
				>
					{loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
					<span>Send</span>
				</button>
			</form>
		</div>
	);
}
