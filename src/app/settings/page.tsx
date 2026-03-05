"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMutation } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { ArrowLeft, Download, Trash2 } from "lucide-react";
import Link from "next/link";
import { useTransport } from "@/components/providers/GrpcProvider";
import { useAuth } from "@/components/providers/AuthProvider";
import { AuthService } from "@/gen/toqui/v1/auth_pb";

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
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 px-4 py-4">
        <div className="max-w-lg mx-auto flex items-center gap-3">
          <Link href="/trips" className="text-gray-500 hover:text-gray-700">
            <ArrowLeft size={20} />
          </Link>
          <h1 className="text-xl font-semibold">{t("title")}</h1>
        </div>
      </header>

      <main className="max-w-lg mx-auto p-4 space-y-4">
        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <h2 className="text-sm font-medium text-gray-500 mb-4">{t("account")}</h2>
          <div className="flex items-center gap-4">
            {user.avatarUrl ? (
              <img
                src={user.avatarUrl}
                alt={user.name}
                className="w-12 h-12 rounded-full"
              />
            ) : (
              <div className="w-12 h-12 rounded-full bg-blue-100 flex items-center justify-center text-blue-600 font-medium text-lg">
                {user.name?.charAt(0)?.toUpperCase() || "?"}
              </div>
            )}
            <div>
              <p className="font-medium text-gray-900">{user.name}</p>
              <p className="text-sm text-gray-500">{user.email}</p>
            </div>
          </div>
        </div>

        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <button
            onClick={() => exportData.mutate()}
            disabled={exportData.isPending}
            className="flex items-center gap-2 text-blue-600 hover:text-blue-700 text-sm font-medium disabled:opacity-50"
          >
            <Download size={16} />
            {exportData.isPending ? t("exporting") : exportData.isSuccess ? t("exported") : t("exportData")}
          </button>
          {exportData.isError && (
            <p className="text-red-600 text-sm mt-2">{tc("error")}</p>
          )}
        </div>

        <div className="bg-white rounded-xl border border-gray-200 p-6">
          {!showDeleteConfirm ? (
            <button
              onClick={() => setShowDeleteConfirm(true)}
              className="flex items-center gap-2 text-red-600 hover:text-red-700 text-sm font-medium"
            >
              <Trash2 size={16} />
              {t("deleteAccount")}
            </button>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-gray-700">{t("deleteWarning")}</p>
              <div>
                <label htmlFor="deleteConfirm" className="block text-sm text-gray-600 mb-1">
                  {t("typeDelete")}
                </label>
                <input
                  id="deleteConfirm"
                  type="text"
                  value={deleteInput}
                  onChange={(e) => setDeleteInput(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-red-500 focus:border-transparent"
                />
              </div>
              <div className="flex gap-3">
                <button
                  onClick={() => deleteAccount.mutate()}
                  disabled={deleteInput !== "DELETE" || deleteAccount.isPending}
                  className="bg-red-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-red-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {deleteAccount.isPending ? t("deleting") : t("deleteConfirm")}
                </button>
                <button
                  onClick={() => {
                    setShowDeleteConfirm(false);
                    setDeleteInput("");
                  }}
                  className="text-gray-500 hover:text-gray-700 text-sm font-medium"
                >
                  {tc("cancel")}
                </button>
              </div>
              {deleteAccount.isError && (
                <p className="text-red-600 text-sm">{tc("error")}</p>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
