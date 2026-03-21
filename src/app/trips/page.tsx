"use client";

import { useEffect, useCallback, useState, useRef } from "react";
import { useAuth } from "@/components/providers/AuthProvider";
import { useTrips } from "@/lib/hooks/useTrips";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ChatContainer } from "@/components/chat/ChatContainer";
import { TripStatus } from "@/gen/toqui/v1/trip_pb";
import type { Trip } from "@/gen/toqui/v1/trip_pb";
import type { CreatedTrip, SelectedTrip } from "@/lib/hooks/useChat";
import Link from "next/link";
import Image from "next/image";
import { MessageSquare, Settings, LogOut, Menu, X, Calendar, MapPin, Plus } from "lucide-react";
import { ThemeToggleButton } from "@/components/theme/ThemeToggle";

const statusLabels: Record<number, string> = {
  [TripStatus.PLANNING]: "planning",
  [TripStatus.ACTIVE]: "traveling",
  [TripStatus.COMPLETED]: "completed",
};

const statusColors: Record<string, string> = {
  planning: "bg-[var(--color-status-planning-bg)] text-[var(--color-status-planning-text)]",
  traveling: "bg-[var(--color-status-active-bg)] text-[var(--color-status-active-text)]",
  completed: "bg-[var(--color-status-completed-bg)] text-[var(--color-status-completed-text)]",
};

export default function TripsPage() {
  const t = useTranslations("trips");
  const { user, isLoading: authLoading, logout } = useAuth();
  const { trips } = useTrips();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/");
    }
  }, [authLoading, user, router]);

  const handleTripCreated = useCallback(
    (trip: CreatedTrip) => {
      void queryClient.invalidateQueries({ queryKey: ["trips"] });
      // Brief delay so the user sees the AI's trip-creation response before
      // navigating to the planning chat. The TripCreated event fires before
      // the AI's follow-up text, so navigating immediately cuts it off.
      setTimeout(() => {
        router.push(`/trips/${trip.id}/chat`);
      }, 1500);
    },
    [queryClient, router],
  );

  const handleTripSelected = useCallback(
    (trip: SelectedTrip) => {
      router.push(`/trips/${trip.id}/chat`);
    },
    [router],
  );

  const sidebarRef = useRef<HTMLElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);

  const closeSidebar = useCallback(() => {
    setSidebarOpen(false);
    // Return focus to the menu button when sidebar closes
    menuButtonRef.current?.focus();
  }, []);

  // Escape key closes the sidebar
  useEffect(() => {
    if (!sidebarOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        closeSidebar();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [sidebarOpen, closeSidebar]);

  // Focus trap for mobile sidebar
  useEffect(() => {
    if (!sidebarOpen || !sidebarRef.current) return;
    const sidebar = sidebarRef.current;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key !== "Tab") return;
      const focusable = sidebar.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    sidebar.addEventListener("keydown", handleKeyDown);
    return () => sidebar.removeEventListener("keydown", handleKeyDown);
  }, [sidebarOpen]);

  // Lock body scroll when mobile sidebar is open
  useEffect(() => {
    if (sidebarOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [sidebarOpen]);

  // Focus the first focusable element inside the sidebar when it opens.
  // Focusing the container itself (tabIndex=-1) doesn't work reliably —
  // the first Tab press would escape the focus trap.
  useEffect(() => {
    if (sidebarOpen && sidebarRef.current) {
      const firstFocusable = sidebarRef.current.querySelector<HTMLElement>(
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])',
      );
      if (firstFocusable) {
        firstFocusable.focus();
      } else {
        sidebarRef.current.focus();
      }
    }
  }, [sidebarOpen]);

  const handleLogout = useCallback(async () => {
    try {
      await logout();
    } finally {
      router.push("/");
    }
  }, [logout, router]);

  if (authLoading || !user) {
    return (
      <div className="h-screen flex items-center justify-center" aria-busy="true" role="status">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)]" />
        <span className="sr-only">Loading...</span>
      </div>
    );
  }

  const userInitial = user.name?.charAt(0)?.toUpperCase() ?? user.email?.charAt(0)?.toUpperCase() ?? "?";

  return (
    <div className="h-screen flex flex-col md:flex-row">
      {/* Mobile top bar */}
      <div className="md:hidden bg-[var(--color-surface-secondary)] border-b border-[var(--color-border)] px-4 py-3 flex items-center justify-between flex-shrink-0">
        <button
          ref={menuButtonRef}
          onClick={() => setSidebarOpen(true)}
          className="p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-tertiary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
          aria-label="Open trip list"
          aria-expanded={sidebarOpen}
          aria-controls="trip-sidebar"
        >
          <Menu size={20} aria-hidden="true" />
        </button>
        <span className="text-base font-semibold text-[var(--color-text-primary)]">Toqui</span>
        {user.avatarUrl ? (
          <Link
            href="/settings"
            className="rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
            aria-label="Settings"
          >
            <Image
              src={user.avatarUrl}
              alt=""
              width={28}
              height={28}
              className="rounded-full"
            />
          </Link>
        ) : (
          <Link
            href="/settings"
            className="w-7 h-7 rounded-full bg-[var(--color-accent-soft)] flex items-center justify-center text-[var(--color-accent)] text-xs font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
            aria-label="Settings"
          >
            {userInitial}
          </Link>
        )}
      </div>

      {/* Backdrop for mobile sidebar */}
      {sidebarOpen && (
        <div
          className="md:hidden fixed inset-0 bg-black/50 z-40"
          onClick={closeSidebar}
          aria-hidden="true"
        />
      )}

      {/* Trip sidebar */}
      <aside
        ref={sidebarRef}
        id="trip-sidebar"
        role={sidebarOpen ? "dialog" : undefined}
        aria-modal={sidebarOpen ? "true" : undefined}
        aria-label={sidebarOpen ? "Trip list" : undefined}
        tabIndex={sidebarOpen ? -1 : undefined}
        className={`
          fixed inset-y-0 left-0 z-50 w-64 bg-[var(--color-surface-secondary)] border-r border-[var(--color-border)] flex flex-col flex-shrink-0
          transform transition-transform duration-300 ease-in-out
          ${sidebarOpen ? "translate-x-0" : "-translate-x-full"}
          md:relative md:translate-x-0 md:transition-none
        `}
      >
        <div className="p-4 border-b border-[var(--color-border)] flex items-center justify-between">
          <h2 className="font-semibold text-sm text-[var(--color-text-secondary)] uppercase tracking-wide">
            {t("title")}
          </h2>
          <button
            onClick={closeSidebar}
            className="md:hidden p-1 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-tertiary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
            aria-label="Close trip list"
          >
            <X size={18} aria-hidden="true" />
          </button>
        </div>
        <div className="px-2 pt-2">
          <Link
            href="/trips/new"
            onClick={closeSidebar}
            className="flex items-center gap-2 w-full rounded-lg px-3 py-2 text-sm font-medium text-[var(--color-accent)] hover:bg-[var(--color-accent-soft)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
          >
            <Plus size={16} aria-hidden="true" />
            {t("newTrip")}
          </Link>
        </div>
        <nav className="flex-1 overflow-y-auto p-2" aria-label="Trip list">
          {trips.length === 0 ? (
            <p className="text-xs text-[var(--color-text-tertiary)] p-2">
              No trips yet. Create one above or start chatting!
            </p>
          ) : (
            trips.map((trip: Trip) => (
              <TripSidebarItem key={trip.id} trip={trip} onSelect={closeSidebar} />
            ))
          )}
        </nav>

        {/* User menu at bottom of sidebar */}
        <div className="border-t border-[var(--color-border)] p-3">
          <div className="flex items-center gap-2">
            {user.avatarUrl ? (
              <Image
                src={user.avatarUrl}
                alt=""
                width={32}
                height={32}
                className="rounded-full flex-shrink-0"
              />
            ) : (
              <div className="w-8 h-8 rounded-full bg-[var(--color-accent-soft)] flex items-center justify-center text-[var(--color-accent)] text-sm font-medium flex-shrink-0">
                {userInitial}
              </div>
            )}
            <span className="text-sm text-[var(--color-text-primary)] truncate flex-1">
              {user.name || user.email}
            </span>
            <Link
              href="/settings"
              className="p-1.5 rounded-lg text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-tertiary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
              aria-label="Settings"
            >
              <Settings size={16} aria-hidden="true" />
            </Link>
            <button
              onClick={handleLogout}
              className="p-1.5 rounded-lg text-[var(--color-text-tertiary)] hover:text-[var(--color-error)] hover:bg-[var(--color-error-bg)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
              aria-label="Sign out"
            >
              <LogOut size={16} aria-hidden="true" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main chat area */}
      <main id="main-content" className="flex-1 flex flex-col min-w-0">
        <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-3 flex-shrink-0 flex items-center gap-3">
          <MessageSquare size={20} className="text-[var(--color-accent)]" aria-hidden="true" />
          <h1 className="text-lg font-semibold text-[var(--color-text-primary)] flex-1">Toqui</h1>
          <ThemeToggleButton />
        </header>
        <ChatContainer
          mode="selection"
          onTripCreated={handleTripCreated}
          onTripSelected={handleTripSelected}
        />
      </main>
    </div>
  );
}

function formatTripDates(start: string, end: string): string {
  if (!start && !end) return "";
  const fmt = (s: string) => {
    const d = new Date(s + "T00:00:00");
    return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  };
  if (start && end) return `${fmt(start)} – ${fmt(end)}`;
  if (start) return `From ${fmt(start)}`;
  return `Until ${fmt(end)}`;
}

function TripSidebarItem({ trip, onSelect }: { trip: Trip; onSelect: () => void }) {
  const label = statusLabels[trip.status] || "planning";
  const colors = statusColors[label] || statusColors.planning;
  const dateStr = formatTripDates(trip.startDate, trip.endDate);

  return (
    <Link
      href={`/trips/${trip.id}`}
      onClick={onSelect}
      className="block rounded-lg p-3 hover:bg-[var(--color-surface-tertiary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
    >
      <div className="flex items-center justify-between mb-0.5">
        <span className="text-sm font-medium text-[var(--color-text-primary)] truncate">
          {trip.title}
        </span>
        <span
          className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium flex-shrink-0 ml-2 ${colors}`}
        >
          {label}
        </span>
      </div>
      {trip.description && (
        <p className="text-xs text-[var(--color-text-tertiary)] truncate mb-1">{trip.description}</p>
      )}
      <div className="flex items-center gap-3 text-[10px] text-[var(--color-text-tertiary)]">
        {dateStr && (
          <span className="flex items-center gap-1">
            <Calendar size={10} aria-hidden="true" />
            {dateStr}
          </span>
        )}
        {trip.destinationCountry && (
          <span className="flex items-center gap-1">
            <MapPin size={10} aria-hidden="true" />
            {trip.destinationCountry.toUpperCase()}
          </span>
        )}
      </div>
    </Link>
  );
}
