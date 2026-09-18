import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { ProfileScreen } from "@/components/profile/profile-screen";
import { ProfilesApiError, fetchProfile, type ProfileRecord } from "@/lib/api/profiles";

export const dynamic = "force-dynamic";

type PublicProfilePageProps = { params: Promise<{ id: string }> };

export async function generateMetadata({ params }: PublicProfilePageProps): Promise<Metadata> {
  const { id } = await params;
  try {
    const profile = await fetchProfile(id);
    return {
      title: profile.display_name || "Profile",
      description: profile.bio?.trim().slice(0, 160) || "Public NIMNear profile",
    };
  } catch {
    return { title: "Profile" };
  }
}

export default async function PublicProfilePage({ params }: PublicProfilePageProps) {
  const { id } = await params;
  let profile: ProfileRecord;
  try {
    profile = await fetchProfile(id);
  } catch (error) {
    if (error instanceof ProfilesApiError && error.status === 404) notFound();
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1000px] px-4 py-8 sm:px-6 lg:px-8"><StateCard kind="error" title="Profile could not be loaded" description="The profile service is currently unavailable." /></main></div>;
  }
  return <ProfileScreen profileId={id} initialProfile={profile} />;
}
