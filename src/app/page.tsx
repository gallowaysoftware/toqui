"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { useAuth } from "@/components/providers/AuthProvider";
import { useRouter } from "next/navigation";
import {
  Compass,
  MessageSquare,
  Calendar,
  Mail,
  Users,
  Map,
  ArrowRight,
} from "lucide-react";

export default function Home() {
  const t = useTranslations("home");
  const { user, isLoading, login } = useAuth();
  const router = useRouter();

  // If logged in, go straight to trips
  useEffect(() => {
    if (!isLoading && user) {
      router.push("/trips");
    }
  }, [isLoading, user, router]);

  if (!isLoading && user) return null;

  return (
    <main id="main-content" className="min-h-screen bg-[var(--color-surface-secondary)]">
      {/* Hero */}
      <section className="flex flex-col items-center justify-center px-6 pt-20 pb-16 text-center">
        <h1 className="text-5xl sm:text-6xl font-bold tracking-tight text-[var(--color-text-primary)] mb-4">
          {t("title")}
        </h1>
        <p className="text-lg sm:text-xl text-[var(--color-text-secondary)] max-w-xl mb-8">
          {t("subtitle")}
        </p>
        <button
          onClick={login}
          className="rounded-full bg-[var(--color-accent)] px-8 py-3.5 text-white font-semibold text-lg hover:bg-[var(--color-accent-hover)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2 flex items-center gap-2"
        >
          {t("startPlanning")}
          <ArrowRight size={20} aria-hidden="true" />
        </button>
      </section>

      {/* Features */}
      <section className="max-w-4xl mx-auto px-6 pb-16">
        <h2 className="text-2xl font-bold text-[var(--color-text-primary)] text-center mb-10">
          {t("featuresTitle")}
        </h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
          <FeatureCard
            icon={Users}
            title={t("feature1Title")}
            description={t("feature1Desc")}
          />
          <FeatureCard
            icon={MessageSquare}
            title={t("feature2Title")}
            description={t("feature2Desc")}
          />
          <FeatureCard
            icon={Calendar}
            title={t("feature3Title")}
            description={t("feature3Desc")}
          />
          <FeatureCard
            icon={Mail}
            title={t("feature4Title")}
            description={t("feature4Desc")}
          />
          <FeatureCard
            icon={Map}
            title={t("feature5Title")}
            description={t("feature5Desc")}
          />
          <FeatureCard
            icon={Compass}
            title={t("feature6Title")}
            description={t("feature6Desc")}
          />
        </div>
      </section>

      {/* How it works */}
      <section className="bg-[var(--color-surface)] border-t border-[var(--color-border)] px-6 py-16">
        <div className="max-w-3xl mx-auto text-center">
          <h2 className="text-2xl font-bold text-[var(--color-text-primary)] mb-10">
            {t("howTitle")}
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-8">
            <StepCard step={1} title={t("step1Title")} description={t("step1Desc")} />
            <StepCard step={2} title={t("step2Title")} description={t("step2Desc")} />
            <StepCard step={3} title={t("step3Title")} description={t("step3Desc")} />
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="px-6 py-16 text-center">
        <h2 className="text-2xl font-bold text-[var(--color-text-primary)] mb-3">
          {t("ctaTitle")}
        </h2>
        <p className="text-[var(--color-text-secondary)] mb-6 max-w-md mx-auto">
          {t("ctaDesc")}
        </p>
        <button
          onClick={login}
          className="rounded-full bg-[var(--color-accent)] px-8 py-3 text-white font-semibold hover:bg-[var(--color-accent-hover)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
        >
          {t("startPlanning")}
        </button>
      </section>

      {/* Footer */}
      <footer className="border-t border-[var(--color-border)] px-6 py-8 text-sm text-[var(--color-text-tertiary)]">
        <div className="max-w-4xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
          <p>&copy; {new Date().getFullYear()} Galloway Software Solutions Inc.</p>
          <div className="flex gap-6">
            <Link href="/privacy" className="hover:text-[var(--color-accent)] transition-colors">
              Privacy Policy
            </Link>
            <Link href="/terms" className="hover:text-[var(--color-accent)] transition-colors">
              Terms of Service
            </Link>
          </div>
        </div>
      </footer>
    </main>
  );
}

function FeatureCard({
  icon: Icon,
  title,
  description,
}: {
  icon: typeof Compass;
  title: string;
  description: string;
}) {
  return (
    <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-5">
      <div className="w-10 h-10 rounded-lg bg-[var(--color-accent-soft)] flex items-center justify-center mb-3">
        <Icon size={20} className="text-[var(--color-accent)]" aria-hidden="true" />
      </div>
      <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-1">{title}</h3>
      <p className="text-sm text-[var(--color-text-secondary)]">{description}</p>
    </div>
  );
}

function StepCard({
  step,
  title,
  description,
}: {
  step: number;
  title: string;
  description: string;
}) {
  return (
    <div className="flex flex-col items-center">
      <div className="w-10 h-10 rounded-full bg-[var(--color-accent)] text-white font-bold text-lg flex items-center justify-center mb-3">
        {step}
      </div>
      <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-1">{title}</h3>
      <p className="text-sm text-[var(--color-text-secondary)]">{description}</p>
    </div>
  );
}
