"use client";

import { useState, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Clock, Mail, Ticket, Users, ArrowRight, CheckCircle } from "lucide-react";
import { useJoinWaitlist, useWaitlistStatus } from "@/lib/hooks/useWaitlist";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8090";

type WaitlistView = "join" | "status" | "invite";

export default function WaitlistPage() {
  const t = useTranslations("waitlist");
  const tc = useTranslations("common");
  const searchParams = useSearchParams();
  const router = useRouter();

  const emailParam = searchParams.get("email");
  const [view, setView] = useState<WaitlistView>("join");
  const [email, setEmail] = useState(emailParam || "");
  const [submittedEmail, setSubmittedEmail] = useState<string | null>(null);
  const [inviteCode, setInviteCode] = useState("");
  const [joinPosition, setJoinPosition] = useState<number | null>(null);

  const joinMutation = useJoinWaitlist();
  const { data: statusData } = useWaitlistStatus(submittedEmail);

  // Position: use polled status if available, otherwise the initial join position
  const position = statusData?.position ?? joinPosition;

  // Redirect when accepted
  useEffect(() => {
    if (statusData?.accepted) {
      router.push(`${API_URL}/auth/google/login`);
    }
  }, [statusData?.accepted, router]);

  const handleJoin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim()) return;

    try {
      const result = await joinMutation.mutateAsync({ email: email.trim() });
      setJoinPosition(result.position);
      setSubmittedEmail(email.trim());
      setView("status");
    } catch {
      // Error is accessible via joinMutation.error
    }
  };

  const handleInviteSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inviteCode.trim()) return;
    window.location.href = `${API_URL}/auth/google/login?invite_code=${encodeURIComponent(inviteCode.trim())}`;
  };

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-8">
      <div className="max-w-lg w-full text-center">
        {/* Branding */}
        <h1 className="text-5xl font-bold tracking-tight text-[var(--color-text-primary)] mb-2">
          {tc("appName")}
        </h1>
        <p className="text-lg text-[var(--color-text-tertiary)] mb-8">
          {tc("tagline")}
        </p>

        {/* Card */}
        <div className="rounded-2xl bg-[var(--color-surface)] border border-[var(--color-border)] p-8 shadow-sm">
          {view === "join" && (
            <>
              <div className="flex justify-center mb-4">
                <div className="rounded-full bg-[var(--color-accent-soft)] p-3">
                  <Users className="h-6 w-6 text-[var(--color-accent)]" />
                </div>
              </div>
              <h2 className="text-2xl font-semibold text-[var(--color-text-primary)] mb-2">
                {t("title")}
              </h2>
              <p className="text-[var(--color-text-secondary)] mb-6">
                {t("description")}
              </p>

              <form onSubmit={handleJoin} className="space-y-4">
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-[var(--color-text-tertiary)]" />
                  <input
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t("emailPlaceholder")}
                    className="w-full rounded-xl border border-[var(--color-input-border)] bg-[var(--color-input-bg)] pl-10 pr-4 py-3 text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent transition-shadow"
                    aria-label={t("emailPlaceholder")}
                    disabled={joinMutation.isPending}
                  />
                </div>

                {joinMutation.isError && (
                  <p className="text-sm text-[var(--color-error)] bg-[var(--color-error-bg)] rounded-lg px-3 py-2" role="alert">
                    {joinMutation.error?.message || tc("error")}
                  </p>
                )}

                <button
                  type="submit"
                  disabled={joinMutation.isPending}
                  className="w-full rounded-xl bg-[var(--color-accent)] px-6 py-3 text-white font-medium hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {joinMutation.isPending ? (
                    <>
                      <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent" />
                      {t("joining")}
                    </>
                  ) : (
                    <>
                      {t("joinButton")}
                      <ArrowRight className="h-4 w-4" />
                    </>
                  )}
                </button>
              </form>

              {/* Invite code link */}
              <div className="mt-6 pt-6 border-t border-[var(--color-border)]">
                <button
                  onClick={() => setView("invite")}
                  className="text-sm text-[var(--color-accent)] hover:underline flex items-center justify-center gap-1 mx-auto"
                >
                  <Ticket className="h-4 w-4" />
                  {t("haveInvite")}
                </button>
              </div>
            </>
          )}

          {view === "status" && (
            <>
              <div className="flex justify-center mb-4">
                <div className="rounded-full bg-[var(--color-success-bg)] p-3">
                  <CheckCircle className="h-6 w-6 text-[var(--color-success)]" />
                </div>
              </div>
              <h2 className="text-2xl font-semibold text-[var(--color-text-primary)] mb-2">
                {t("joinedTitle")}
              </h2>
              <p className="text-[var(--color-text-secondary)] mb-6">
                {t("joinedDescription")}
              </p>

              {position !== null && (
                <div className="rounded-xl bg-[var(--color-surface-tertiary)] p-4 mb-6">
                  <div className="flex items-center justify-center gap-2 mb-1">
                    <Clock className="h-4 w-4 text-[var(--color-text-tertiary)]" />
                    <span className="text-sm text-[var(--color-text-secondary)]">
                      {t("positionLabel")}
                    </span>
                  </div>
                  <p className="text-3xl font-bold text-[var(--color-accent)]" data-testid="waitlist-position">
                    #{position}
                  </p>
                  <p className="text-sm text-[var(--color-text-tertiary)] mt-1">
                    {t("estimatedWait")}
                  </p>
                </div>
              )}

              <p className="text-sm text-[var(--color-text-tertiary)]">
                {t("notifyMessage")}
              </p>

              {/* Invite code link */}
              <div className="mt-6 pt-6 border-t border-[var(--color-border)]">
                <button
                  onClick={() => setView("invite")}
                  className="text-sm text-[var(--color-accent)] hover:underline flex items-center justify-center gap-1 mx-auto"
                >
                  <Ticket className="h-4 w-4" />
                  {t("haveInvite")}
                </button>
              </div>
            </>
          )}

          {view === "invite" && (
            <>
              <div className="flex justify-center mb-4">
                <div className="rounded-full bg-[var(--color-accent-soft)] p-3">
                  <Ticket className="h-6 w-6 text-[var(--color-accent)]" />
                </div>
              </div>
              <h2 className="text-2xl font-semibold text-[var(--color-text-primary)] mb-2">
                {t("inviteTitle")}
              </h2>
              <p className="text-[var(--color-text-secondary)] mb-6">
                {t("inviteDescription")}
              </p>

              <form onSubmit={handleInviteSubmit} className="space-y-4">
                <input
                  type="text"
                  required
                  value={inviteCode}
                  onChange={(e) => setInviteCode(e.target.value)}
                  placeholder={t("inviteCodePlaceholder")}
                  className="w-full rounded-xl border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-4 py-3 text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent transition-shadow text-center tracking-widest font-mono"
                  aria-label={t("inviteCodePlaceholder")}
                />

                <button
                  type="submit"
                  className="w-full rounded-xl bg-[var(--color-accent)] px-6 py-3 text-white font-medium hover:bg-[var(--color-accent-hover)] transition-colors flex items-center justify-center gap-2"
                >
                  {t("redeemButton")}
                  <ArrowRight className="h-4 w-4" />
                </button>
              </form>

              <button
                onClick={() => setView(submittedEmail ? "status" : "join")}
                className="mt-4 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
              >
                {tc("back")}
              </button>
            </>
          )}
        </div>
      </div>
    </main>
  );
}
