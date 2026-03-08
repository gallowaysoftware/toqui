"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMutation } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { ArrowLeft, Download, Trash2 } from "lucide-react";
import Link from "next/link";
import Image from "next/image";
import { useTransport } from "@/components/providers/GrpcProvider";
import { useAuth } from "@/components/providers/AuthProvider";
import { AuthService } from "@/gen/toqui/v1/auth_pb";
import { ThemeSelector } from "@/components/theme/ThemeToggle";

export default function SettingsPage() {
  const t = useTranslations("settings");
  const tc = useTranslations("common");
  const { user, isLoading: authLoading, logout } = useAuth();
  const router = useRouter();
  const transport = useTransport();
  const client = createClient(AuthService, transport);

  const [deleteInput, setDeleteInput] = useState("");
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  const deleteAccount = useMutation({
    mutationFn: async () => {
      await client.deleteAccount({ confirm: true });
    },
    onSuccess: () => {
      logout();
      router.push("/");
    },
  });

  const exportData = useMutation({
    mutationFn: async () => {
      const res = await client.exportData({});
      return res;
    },
  });

  if (authLoading) return null;
  if (!user) {
    router.push("/");
    return null;
  }

  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)]">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-4">
        <div className="max-w-lg mx-auto flex items-center gap-3">
          <Link
            href="/trips"
            className="text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
            aria-label="Back to trips"
          >
            <ArrowLeft size={20} aria-hidden="true" />
          </Link>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{t("title")}</h1>
        </div>
      </header>

      <main id="main-content" className="max-w-lg mx-auto p-4 space-y-4">
        <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
          <h2 className="text-sm font-medium text-[var(--color-text-secondary)] mb-4">
            {t("account")}
          </h2>
          <div className="flex items-center gap-4">
            {user.avatarUrl ? (
              <Image
                src={user.avatarUrl}
                alt={`${user.name}'s profile photo`}
                width={48}
                height={48}
                className="rounded-full"
                unoptimized
              />
            ) : (
              <div className="w-12 h-12 rounded-full bg-[var(--color-accent-soft)] flex items-center justify-center text-[var(--color-accent)] font-medium text-lg">
                {user.name?.charAt(0)?.toUpperCase() ?? "?"}
              </div>
            )}
            <div>
              <p className="font-medium text-[var(--color-text-primary)]">{user.name}</p>
              <p className="text-sm text-[var(--color-text-secondary)]">{user.email}</p>
            </div>
          </div>
        </div>

        {/* Appearance */}
        <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
          <h2 className="text-sm font-medium text-[var(--color-text-secondary)] mb-4">
            Appearance
          </h2>
          <ThemeSelector />
        </div>

        <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
          <button
            onClick={() => exportData.mutate()}
            disabled={exportData.isPending}
            className="flex items-center gap-2 text-[var(--color-accent)] hover:opacity-80 text-sm font-medium disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
          >
            <Download size={16} aria-hidden="true" />
            {exportData.isPending
              ? t("exporting")
              : exportData.isSuccess
                ? t("exported")
                : t("exportData")}
          </button>
          {exportData.isError && (
            <p className="text-[var(--color-error)] text-sm mt-2" role="alert">
              {tc("error")}
            </p>
          )}
        </div>

        <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
          {!showDeleteConfirm ? (
            <button
              onClick={() => setShowDeleteConfirm(true)}
              className="flex items-center gap-2 text-[var(--color-error)] hover:opacity-80 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
            >
              <Trash2 size={16} aria-hidden="true" />
              {t("deleteAccount")}
            </button>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-[var(--color-text-secondary)]">{t("deleteWarning")}</p>
              <div>
                <label
                  htmlFor="deleteConfirm"
                  className="block text-sm text-[var(--color-text-secondary)] mb-1"
                >
                  {t("typeDelete")}
                </label>
                <input
                  id="deleteConfirm"
                  type="text"
                  value={deleteInput}
                  onChange={(e) => setDeleteInput(e.target.value)}
                  className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-error)] focus:border-transparent"
                />
              </div>
              <div className="flex gap-3">
                <button
                  onClick={() => deleteAccount.mutate()}
                  disabled={deleteInput !== "DELETE" || deleteAccount.isPending}
                  className="bg-[var(--color-error)] text-white px-4 py-2 rounded-lg text-sm font-medium hover:opacity-90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {deleteAccount.isPending ? t("deleting") : t("deleteConfirm")}
                </button>
                <button
                  onClick={() => {
                    setShowDeleteConfirm(false);
                    setDeleteInput("");
                  }}
                  className="text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] text-sm font-medium"
                >
                  {tc("cancel")}
                </button>
              </div>
              {deleteAccount.isError && (
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
