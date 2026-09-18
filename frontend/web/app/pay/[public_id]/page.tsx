import type { Metadata } from "next";

import { AppHeader } from "@/components/app/app-header";
import { PayRequestScreen } from "@/components/payment-requests/pay-request-screen";

export const dynamic = "force-dynamic";

type PayPageProps = {
  params: Promise<{ public_id: string }>;
};

export const metadata: Metadata = {
  title: "Payment request · NIMNear",
  description: "Review and pay a NIMNear payment request.",
};

export default async function PayPage({ params }: PayPageProps) {
  const { public_id } = await params;
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <PayRequestScreen publicId={public_id} />
    </div>
  );
}
