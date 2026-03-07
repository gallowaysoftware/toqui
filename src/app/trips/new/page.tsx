"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useCreateTrip } from "@/lib/hooks/useTrips";
import { useAuth } from "@/components/providers/AuthProvider";

export default function NewTripPage() {
  const t = useTranslations("tripCreate");
  const tc = useTranslations("common");
  const { user, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const createTrip = useCreateTrip();

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  if (authLoading) return null;
  if (!user) {
    router.push("/");
    return null;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;

    const trip = await createTrip.mutateAsync({
      title: title.trim(),
      description: description.trim() || undefined,
      startDate: startDate || undefined,
      endDate: endDate || undefined,
    });

    if (trip) {
      router.push(`/trips/${trip.id}/chat`);
    } else {
      router.push("/trips");
    }
  };

  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)]">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-4">
        <div className="max-w-lg mx-auto flex items-center gap-3">
          <Link href="/trips" className="text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded" aria-label="Back to trips">
            <ArrowLeft size={20} aria-hidden="true" />
          </Link>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{t("title")}</h1>
        </div>
      </header>

      <main id="main-content" className="max-w-lg mx-auto p-4">
        <form onSubmit={handleSubmit} className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6 space-y-5">
          <div>
            <label htmlFor="title" className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">
              {t("whereLabel")}
            </label>
            <input
              id="title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t("wherePlaceholder")}
              className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
              autoFocus
              required
            />
          </div>

          <div>
            <label htmlFor="description" className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">
              {t("descriptionLabel")}
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t("descriptionPlaceholder")}
              rows={3}
              className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent resize-none"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="startDate" className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">
                {t("startDate")}
              </label>
              <input
                id="startDate"
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
                className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
              />
            </div>
            <div>
              <label htmlFor="endDate" className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1">
                {t("endDate")}
              </label>
              <input
                id="endDate"
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
                min={startDate || undefined}
                className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={!title.trim() || createTrip.isPending}
            className="w-full bg-[var(--color-accent)] text-white py-2.5 rounded-lg font-medium hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-sm"
          >
            {createTrip.isPending ? t("submitting") : t("submit")}
          </button>

          {createTrip.isError && (
            <p className="text-[var(--color-error)] text-sm text-center" role="alert">{tc("error")}</p>
          )}
        </form>
      </main>
    </div>
  );
}
