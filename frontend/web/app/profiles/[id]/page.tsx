import { notFound } from "next/navigation";

import { AppHeader } from "@/components/app/app-header";
import { ProfileScreen } from "@/components/profile/profile-screen";
import { ProfilesApiError, fetchProfile, type ProfileRecord } from "@/lib/api/profiles";

export const dynamic = "force-dynamic";

type PublicProfilePageProps = { params: Promise<{ id: string }> };

export default async function PublicProfilePage({ params }: PublicProfilePageProps) {
  const { id } = await params;
  let profile: ProfileRecord;
  try {
    profile = await fetchProfile(id);
  } catch (error) {
    if (error instanceof ProfilesApiError && error.status === 404) notFound();
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1000px] px-4 py-8 sm:px-6 lg:px-8"><p className="text-sm text-muted">Profil yüklenemedi.</p></main></div>;
  }
  return <ProfileScreen profileId={id} initialProfile={profile} />;
}
