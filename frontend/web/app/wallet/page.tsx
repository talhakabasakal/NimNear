import type { Metadata } from "next";

import { AppHeader } from "@/components/app/app-header";
import { WalletScreen } from "@/components/wallet/wallet-screen";

export const metadata: Metadata = {
  title: "Wallet · NIMNear",
  description: "View your Nimiq wallet balance and recent activity.",
};

export default function WalletPage() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <WalletScreen />
    </div>
  );
}
