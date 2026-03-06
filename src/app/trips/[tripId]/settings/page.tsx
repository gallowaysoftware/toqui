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
      queryClient.invalidateQueries({ queryKey: ["trip", tripId] });
      queryClient.invalidateQueries({ queryKey: ["trips"] });
    },
  });

  const deleteTrip = useMutation({
    mutationFn: async () => {
      await client.deleteTrip({ id: tripId });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["trips"] });
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
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 px-4 py-4">
        <div className="max-w-lg mx-auto flex items-center gap-3">
          <Link href={`/trips/${tripId}`} className="text-gray-500 hover:text-gray-700">
            <ArrowLeft size={20} />
          </Link>
          <h1 className="text-xl font-semibold">{t("title")}</h1>
        </div>
      </header>

      <main className="max-w-lg mx-auto p-4 space-y-4">
        <form onSubmit={handleSubmit} className="bg-white rounded-xl border border-gray-200 p-6 space-y-5">
          <div>
            <label htmlFor="title" className="block text-sm font-medium text-gray-700 mb-1">
              {t("editTitle")}
            </label>
            <input
              id="title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              required
            />
          </div>

          <div>
            <label htmlFor="description" className="block text-sm font-medium text-gray-700 mb-1">
              {t("editDescription")}
            </label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="startDate" className="block text-sm font-medium text-gray-700 mb-1">
                {t("editStartDate")}
              </label>
              <input
                id="startDate"
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>
            <div>
              <label htmlFor="endDate" className="block text-sm font-medium text-gray-700 mb-1">
                {t("editEndDate")}
              </label>
              <input
                id="endDate"
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
                min={startDate || undefined}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={!title.trim() || updateTrip.isPending}
            className="w-full bg-blue-600 text-white py-2.5 rounded-lg font-medium hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed text-sm"
          >
            {updateTrip.isPending ? t("saving") : t("save")}
          </button>

          {updateTrip.isError && (
            <p className="text-red-600 text-sm text-center">{tc("error")}</p>
          )}
        </form>

        <div className="bg-white rounded-xl border border-gray-200 p-6">
          {!showDeleteConfirm ? (
            <button
              onClick={() => setShowDeleteConfirm(true)}
              className="flex items-center gap-2 text-red-600 hover:text-red-700 text-sm font-medium"
            >
              <Trash2 size={16} />
              {t("deleteTrip")}
            </button>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-gray-700">{t("deleteWarning")}</p>
              <div className="flex gap-3">
                <button
                  onClick={() => deleteTrip.mutate()}
                  disabled={deleteTrip.isPending}
                  className="bg-red-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-red-700 transition-colors disabled:opacity-50"
                >
                  {deleteTrip.isPending ? t("deleting") : t("deleteConfirm")}
                </button>
                <button
                  onClick={() => setShowDeleteConfirm(false)}
                  className="text-gray-500 hover:text-gray-700 text-sm font-medium"
                >
                  {tc("cancel")}
                </button>
              </div>
              {deleteTrip.isError && (
                <p className="text-red-600 text-sm">{tc("error")}</p>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
