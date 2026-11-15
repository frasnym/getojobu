import { redirect } from "next/navigation";

interface JobDetailPageProps {
  params: Promise<{ slug: string }>;
}

export default async function JobDetailPage({ params }: JobDetailPageProps) {
  const { slug } = await params;
  redirect(`/user-jobs/${slug}`);
}
