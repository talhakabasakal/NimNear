"use client";

import Link from "next/link";
import { Check, Copy } from "lucide-react";
import { useMemo, useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { NimiqIdenticon } from "@/components/profile/nimiq-identicon";
import {
  clearAuthSession,
  deleteAccount,
  logout,
  type AuthSession,
} from "@/lib/api/auth";
import { userFacingCaughtError } from "@/lib/api/http-error";
import {
  ProfilesApiError,
  updateProfile,
  type ProfileRecord,
} from "@/lib/api/profiles";
import { nimiqNetworkLabel } from "@/lib/auth/nimiq-network";
import {
  compactNimiqAddress,
  identiconSeeds,
  shortenNimiqAddress,
} from "@/lib/nimiq/address";
import { cn } from "@/lib/utils";

const joinedFormatter = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
  year: "numeric",
});

const inputClass =
  "mt-1.5 h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30";

const INITIAL_FACE_COUNT = 18;
const MORE_FACE_COUNT = 36;

type AccountProfileProps = {
  profile: ProfileRecord;
  session: AuthSession;
  onProfileChange: (profile: ProfileRecord) => void;
};

function profileName(profile: ProfileRecord) {
  return (
    profile.display_name.trim() ||
    (profile.wallet_address ? shortenNimiqAddress(profile.wallet_address) : "") ||
    "NIMNear user"
  );
}

function joinedLabel(joinedAt: string) {
  const date = new Date(joinedAt);
  return Number.isNaN(date.getTime())
    ? "Date unavailable"
    : joinedFormatter.format(date);
}

function fieldMessage(error: ProfilesApiError) {
  switch (error.errorCode) {
    case "invalid_display_name":
      return "Display name must be 1–100 characters and cannot contain line breaks.";
    case "invalid_bio":
      return "The headline contains unsupported control characters.";
    case "bio_too_long":
      return "Headline cannot exceed 280 characters.";
    case "empty_profile_update":
      return "Change at least one field to save.";
    default:
      return error.message || "Profile could not be updated. Try again.";
  }
}

export function AccountProfile({
  profile,
  session,
  onProfileChange,
}: AccountProfileProps) {
  const walletAddress = profile.wallet_address?.trim() || session.user.wallet_address?.trim() || "";
  const compactAddress = walletAddress ? compactNimiqAddress(walletAddress) : "";
  const [displayName, setDisplayName] = useState(profile.display_name);
  const [headline, setHeadline] = useState(profile.bio ?? "");
  const [copied, setCopied] = useState(false);
  const [pending, setPending] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [showMoreFaces, setShowMoreFaces] = useState(false);
  const [selectedFace, setSelectedFace] = useState(0);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deletePending, setDeletePending] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const faces = useMemo(
    () =>
      compactAddress
        ? identiconSeeds(compactAddress, showMoreFaces ? MORE_FACE_COUNT : INITIAL_FACE_COUNT)
        : [],
    [compactAddress, showMoreFaces],
  );

  const previewSeed = faces[selectedFace] ?? compactAddress;
  const name = displayName.trim() || profileName(profile);
  const networkLabel = nimiqNetworkLabel();

  async function copyAddress() {
    if (!compactAddress) return;
    try {
      await navigator.clipboard.writeText(compactAddress);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  }

  async function signOut() {
    await logout().catch(() => undefined);
    clearAuthSession();
    window.location.reload();
  }

  async function confirmDeleteAccount() {
    if (deletePending) return;
    setDeletePending(true);
    setDeleteError(null);
    try {
      await deleteAccount();
      clearAuthSession();
      window.location.assign("/");
    } catch (error) {
      setDeleteError(userFacingCaughtError(error, "Account could not be deleted. Try again."));
      setDeletePending(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const nextName = displayName.trim();
    const nextHeadline = headline.replace(/\r\n?/g, "\n");
    const nameChanged = nextName !== profile.display_name.trim();
    const headlineChanged = nextHeadline !== (profile.bio ?? "");

    if (!nameChanged && !headlineChanged) {
      setFormError("Change at least one field to save.");
      return;
    }

    setPending(true);
    setFormError(null);
    try {
      const updated = await updateProfile(
        {
          ...(nameChanged ? { display_name: nextName || null } : {}),
          ...(headlineChanged ? { bio: nextHeadline || null } : {}),
        },
      );
      onProfileChange(updated);
      setDisplayName(updated.display_name);
      setHeadline(updated.bio ?? "");
    } catch (error) {
      if (error instanceof ProfilesApiError && error.errorCode) {
        setFormError(fieldMessage(error));
      } else {
        setFormError(userFacingCaughtError(error, "Profile could not be updated. Try again."));
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="space-y-8">
      <header>
        <p className="text-[11px] font-medium uppercase tracking-[0.16em] text-muted">
          Your account
        </p>
        <h1 className="mt-1 text-[28px] font-semibold tracking-[-0.03em] text-foreground">
          Profile
        </h1>
        <p className="mt-2 max-w-xl text-sm leading-6 text-muted">
          Your Nimiq wallet is your NIMNear account. No username, no password.
        </p>
      </header>

      <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
        <div className="flex items-start gap-3">
          {compactAddress ? (
            <NimiqIdenticon seed={compactAddress} className="size-12 shrink-0" alt="" />
          ) : null}
          <div className="min-w-0 flex-1">
            <p className="text-[17px] font-semibold tracking-[-0.02em] text-foreground">
              {compactAddress ? shortenNimiqAddress(compactAddress) : name}
            </p>
            <p className="mt-0.5 text-sm text-muted">
              {compactAddress
                ? [
                    "Logged in with your wallet",
                    networkLabel,
                    "Joined " + joinedLabel(profile.joined_at),
                  ]
                    .filter(Boolean)
                    .join(" · ")
                : "Joined " + joinedLabel(profile.joined_at)}
            </p>
          </div>
          <Button type="button" variant="ghost" size="sm" onClick={() => void signOut()}>
            Sign out
          </Button>
        </div>

        {compactAddress ? (
          <>
            <div className="mt-4 flex items-center gap-3 rounded-xl bg-background px-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted">
                  Wallet address
                </p>
                <p className="mt-0.5 truncate font-mono text-[13px] text-foreground">
                  {compactAddress}
                </p>
              </div>
              <button
                type="button"
                onClick={() => void copyAddress()}
                className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border border-border bg-surface px-3 text-xs font-medium text-foreground transition-colors hover:bg-surface-hover"
              >
                {copied ? <Check size={13} /> : <Copy size={13} />}
                {copied ? "Copied" : "Copy"}
              </button>
            </div>
            <Link
              href="/wallet"
              className="mt-3 inline-flex text-xs font-medium text-accent transition-colors hover:text-foreground"
            >
              View wallet
            </Link>
          </>
        ) : null}
      </section>

      <hr className="border-border" />

      <section>
        <h2 className="text-xl font-semibold tracking-[-0.025em] text-foreground">
          Public profile
        </h2>
        <div className="mt-4 rounded-xl border border-border bg-surface p-5 sm:p-6">
          <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
            <div>
              <div className="flex items-start gap-3">
                {previewSeed ? (
                  <NimiqIdenticon seed={previewSeed} className="size-12 shrink-0" alt="" />
                ) : null}
                <div className="min-w-0">
                  <p className="text-[16px] font-semibold tracking-[-0.02em] text-foreground">
                    {name}
                  </p>
                  <p className="mt-1 text-sm leading-5 text-muted">
                    This is how you appear as the organizer on events you publish.
                  </p>
                </div>
              </div>

              <form className="mt-6 space-y-4" onSubmit={handleSubmit}>
                <label className="block text-xs font-medium text-foreground">
                  Display name
                  <input
                    className={inputClass}
                    value={displayName}
                    onChange={(event) => {
                      setDisplayName(event.target.value);
                      setFormError(null);
                    }}
                    autoComplete="name"
                    maxLength={100}
                  />
                  <span className="mt-1.5 block text-xs leading-5 text-muted">
                    The name on your events. For example, “Alex Fitness”.
                  </span>
                </label>

                <label className="block text-xs font-medium text-foreground">
                  Headline{" "}
                  <span className="font-normal text-muted">Optional</span>
                  <input
                    className={inputClass}
                    value={headline}
                    onChange={(event) => {
                      setHeadline(event.target.value);
                      setFormError(null);
                    }}
                    maxLength={280}
                    placeholder=""
                  />
                </label>

                {formError ? (
                  <p className="text-xs text-red-200" role="alert">
                    {formError}
                  </p>
                ) : null}

                <Button type="submit" disabled={pending}>
                  {pending ? "Saving…" : "Save changes"}
                </Button>
              </form>
            </div>

            <div>
              <p className="text-sm font-medium text-foreground">Your face</p>
              <p className="mt-1 text-sm leading-5 text-muted">
                Nimiq identicons of your wallet. The first one is your wallet's own.
              </p>
              {faces.length > 0 ? (
                <>
                  <div className="mt-4 grid grid-cols-6 gap-x-2 gap-y-3 sm:gap-x-3">
                    {faces.map((seed, index) => (
                      <button
                        key={seed}
                        type="button"
                        aria-label={index === 0 ? "Wallet identicon" : "Identicon " + (index + 1)}
                        aria-pressed={selectedFace === index}
                        onClick={() => setSelectedFace(index)}
                        className={cn(
                          "grid aspect-square place-items-center rounded-full p-0.5 transition-shadow",
                          selectedFace === index
                            ? "ring-2 ring-foreground ring-offset-2 ring-offset-surface"
                            : "hover:ring-1 hover:ring-border-faint",
                        )}
                      >
                        <NimiqIdenticon seed={seed} className="size-full" alt="" />
                      </button>
                    ))}
                  </div>
                  <button
                    type="button"
                    className="mt-4 text-sm font-medium text-foreground underline underline-offset-4"
                    onClick={() => setShowMoreFaces((current) => !current)}
                  >
                    {showMoreFaces ? "Show less" : "Show more"}
                  </button>
                </>
              ) : (
                <p className="mt-4 text-sm text-muted">
                  Connect a Nimiq wallet to see identicons for this account.
                </p>
              )}
            </div>
          </div>
        </div>
      </section>

      <section className="rounded-xl border border-red-300/20 bg-surface p-5 sm:p-6">
        <h2 className="text-xl font-semibold tracking-[-0.025em] text-foreground">Account</h2>
        <p className="mt-2 max-w-xl text-sm leading-6 text-muted">
          Sign out of this browser, or delete your NIMNear account. Deletion
          removes your public profile details. Payment and event evidence stay
          on the server for integrity.
        </p>
        <div className="mt-4 flex flex-wrap gap-3">
          <Button type="button" variant="outline" onClick={() => void signOut()}>
            Log out
          </Button>
          <Button
            type="button"
            variant="outline"
            className="border-red-300/40 text-red-200 hover:bg-red-400/10"
            onClick={() => {
              setDeleteError(null);
              setDeleteOpen(true);
            }}
          >
            Delete account
          </Button>
        </div>
      </section>

      <ConfirmDialog
        open={deleteOpen}
        title="Delete this account?"
        description="This cannot be undone from the app. Your public name, bio, and wallet login will be removed. Existing events, purchases, and payment hashes remain for evidence."
        confirmLabel="Delete account"
        pending={deletePending}
        pendingLabel="Deleting…"
        destructive
        error={deleteError}
        onConfirm={() => {
          void confirmDeleteAccount();
        }}
        onOpenChange={(open) => {
          if (!deletePending) {
            setDeleteOpen(open);
            if (!open) setDeleteError(null);
          }
        }}
      />
    </div>
  );
}
