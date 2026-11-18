import type { Metadata } from "next";
import { ClerkProvider } from "@clerk/nextjs";
import { ThemeProvider } from "@/components/theme-provider";
import "./globals.css";

export const metadata: Metadata = {
  title: "getojobu - Job Match Reports & Resumes",
  description: "AI job compatibility evaluation, match reports, and tailored resumes.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const publishableKey =
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ||
    "pk_test_ZXhhbXBsZS5jbGVyay5hY2NvdW50cy5kZXYk";

  return (
    <ClerkProvider publishableKey={publishableKey}>
      <html lang="en" suppressHydrationWarning>
        <body className="min-h-screen bg-background font-sans antialiased text-foreground">
          <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
            {children}
          </ThemeProvider>
        </body>
      </html>
    </ClerkProvider>
  );
}
