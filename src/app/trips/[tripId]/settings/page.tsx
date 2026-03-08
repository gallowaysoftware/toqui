"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { ArrowLeft, Trash2 } from "lucide-react";
import Link from "next/link";
import { useTransport } from "@/components/providers/GrpcProvider";
import { useAuth } from "@/components/providers/AuthProvider";
import { TripService } from "@/gen/toqui/v1/trip_pb";

export default function TripSettingsPage() {
  const t = useTranslations("tripSettings");
  const tc = useTranslations("common");
  const { tripId } = useParams<{ tripId: string }>();
  const { user, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const transport = useTransport();
  const queryClient = useQueryClient();
  const client = createClient(TripService, transport);

  const { data: trip, isLoading } = useQuery({
    queryKey: ["trip", tripId],
    queryFn: async () => {
      const res = await client.getTrip({ id: tripId });
      return res.trip;
    },
    enabled: !!user,
  });

  const [title, setTitle] = useState(trip?.title ?? "");
  const [description, setDescription] = useState(trip?.description ?? "");
  const [startDate, setStartDate] = useState(trip?.startDate ?? "");
  const [endDate, setEndDate] = useState(trip?.endDate ?? "");
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [initialized, setInitialized] = useState(false);

  // Sync form state once when trip data arrives (without useEffect + setState)
  if (trip && !initialized) {
    setTitle(trip.title);
    setDescription(trip.description);
    setStartDate(trip.startDate);
    setEndDate(trip.endDate);
    setInitialized(true);
  }

  const updateTrip = useMutation({
    mutationFn: async () => {
      await client.updateTrip({
        id: tripId,
        title: title.trim(),
        description: description.trim(),
        startDate,
        endDate,
      });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["trip", tripId] });
      void queryClient.invalidateQueries({ queryKey: ["trips"] });
    },
  });

  const deleteTrip = useMutation({
    mutationFn: async () => {
      await client.deleteTrip({ id: tripId });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["trips"] });
      router.push("/trips");
    },
  });

  if (authLoading || isLoading) return null;
  if (!user) {
    router.push("/");
    return null;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;
    await updateTrip.mutateAsync();
  };

  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)]">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-4">
        <div className="max-w-lg mx-auto flex items-center gap-3">
          <Link
            href={`/trips/${tripId}`}
            className="text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
            aria-label="Back to trip"
          >
            <ArrowLeft size={20} aria-hidden="true" />
          </Link>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{t("title")}</h1>
        </div>
      </header>

      <main id="main-content" className="max-w-lg mx-auto p-4 space-y-4">
        <form
          onSubmit={handleSubmit}
          className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6 space-y-5"
        >
          <div>
            <label
              htmlFor="title"
              className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
            >
              {t("editTitle")}
            </label>
            <input
              id="title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
              required
            />
          </div>

          <div>
            <label
              htmlFor="description"
              className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
            >
              {t("editDescription")}
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent resize-none"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label
                htmlFor="startDate"
                className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
              >
                {t("editStartDate")}
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
              <label
                htmlFor="endDate"
                className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
              >
                {t("editEndDate")}
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
            disabled={!title.trim() || updateTrip.isPending}
            className="w-full bg-[var(--color-accent)] text-white py-2.5 rounded-lg font-medium hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-sm"
          >
            {updateTrip.isPending ? t("saving") : t("save")}
          </button>

          {updateTrip.isError && (
            <p className="text-[var(--color-error)] text-sm text-center" role="alert">
              {tc("error")}
            </p>
          )}
        </form>

        <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
          {!showDeleteConfirm ? (
            <button
              onClick={() => setShowDeleteConfirm(true)}
              className="flex items-center gap-2 text-[var(--color-error)] hover:opacity-80 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
            >
              <Trash2 size={16} aria-hidden="true" />
              {t("deleteTrip")}
            </button>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-[var(--color-text-secondary)]">{t("deleteWarning")}</p>
              <div className="flex gap-3">
                <button
                  onClick={() => deleteTrip.mutate()}
                  disabled={deleteTrip.isPending}
                  className="bg-[var(--color-error)] text-white px-4 py-2 rounded-lg text-sm font-medium hover:opacity-90 transition-colors disabled:opacity-50"
                >
                  {deleteTrip.isPending ? t("deleting") : t("deleteConfirm")}
                </button>
                <button
                  onClick={() => setShowDeleteConfirm(false)}
                  className="text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] text-sm font-medium"
                >
                  {tc("cancel")}
                </button>
              </div>
              {deleteTrip.isError && (
                <p className="text-[var(--color-error)] text-sm" role="alert">
                  {tc("error")}
                </p>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
