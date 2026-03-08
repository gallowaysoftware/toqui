import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Privacy Policy — Toqui",
  description: "How Toqui collects, uses, and protects your personal information.",
};

export default function PrivacyPolicyPage() {
  return (
    <main id="main-content" className="min-h-screen bg-[var(--color-surface-secondary)]">
      <div className="mx-auto max-w-3xl px-6 py-16">
        <Link
          href="/"
          className="text-sm text-[var(--color-text-tertiary)] hover:text-[var(--color-accent)] transition-colors mb-8 inline-block"
        >
          ← Back to Toqui
        </Link>

        <article className="prose prose-neutral dark:prose-invert max-w-none">
          <h1 className="text-3xl font-bold tracking-tight text-[var(--color-text-primary)] mb-2">
            Privacy Policy
          </h1>
          <p className="text-sm text-[var(--color-text-tertiary)] mb-8">
            Last updated: March 8, 2026
          </p>

          <section className="space-y-4 text-[var(--color-text-secondary)]">
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              1. Who We Are
            </h2>
            <p>
              Toqui is an AI-powered travel planning service operated by Galloway Software Solutions
              Inc. (&quot;we,&quot; &quot;us,&quot; or &quot;our&quot;). This policy explains how we
              collect, use, and protect your information when you use the Toqui application.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              2. Information We Collect
            </h2>
            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              Account Information
            </h3>
            <p>
              When you sign in with Google, we receive your name, email address, and profile
              picture. We use this to create and maintain your account.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              Trip &amp; Chat Data
            </h3>
            <p>
              We store the trips you create, itinerary items, bookings, and chat messages so you can
              access them across sessions. Chat messages are sent to third-party AI providers (see
              Section 5) to generate responses.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              Usage Data
            </h3>
            <p>
              We collect basic usage metrics such as message counts and feature usage to improve the
              service. We do not use third-party analytics trackers.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              3. How We Use Your Information
            </h2>
            <ul className="list-disc pl-6 space-y-1">
              <li>Provide and personalize the travel planning service</li>
              <li>Generate AI-powered recommendations and itineraries</li>
              <li>Maintain and improve the application</li>
              <li>Enforce our Terms of Service and prevent abuse</li>
              <li>Communicate service updates when necessary</li>
            </ul>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              4. AI-Generated Content
            </h2>
            <p>
              Toqui uses artificial intelligence to generate travel recommendations, itinerary
              suggestions, and conversational responses. AI-generated content may not always be
              accurate or up-to-date. You should verify important details — such as prices,
              availability, visa requirements, and safety information — before making travel
              decisions.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              5. Third-Party Services
            </h2>
            <p>We use the following third-party services to operate Toqui:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                <strong>Google Cloud Platform</strong> — hosting, database storage, and
                authentication
              </li>
              <li>
                <strong>Anthropic (Claude)</strong> and <strong>Google Vertex AI (Gemini)</strong> —
                AI language models that process your chat messages to generate responses
              </li>
              <li>
                <strong>Affiliate partners</strong> (e.g., Booking.com, GetYourGuide) — when you
                click booking links, those partners may collect data per their own privacy policies
              </li>
            </ul>
            <p>
              Your chat messages are sent to AI providers to generate responses. We do not sell your
              data to any third party.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              6. Cookies &amp; Local Storage
            </h2>
            <p>
              We use a secure, HTTP-only cookie solely to maintain your authentication session. This
              cookie does not track you across other websites. We also use your browser&apos;s local
              storage to remember preferences (such as dark/light mode) and age verification status.
              We do not use third-party tracking cookies or analytics.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              7. Data Storage &amp; Security
            </h2>
            <p>
              Your data is stored on Google Cloud Platform servers. We use encryption in transit
              (TLS) and at rest. Access to production systems is restricted to authorized personnel.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              8. Data Retention
            </h2>
            <p>
              We retain your account data and trip history for as long as your account is active.
              Chat messages are retained to provide conversation continuity. You may request
              deletion of your data at any time by contacting us.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              9. Your Rights
            </h2>
            <p>You have the right to:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Access the personal data we hold about you</li>
              <li>Request correction of inaccurate data</li>
              <li>Request deletion of your data</li>
              <li>Export your trip and itinerary data</li>
            </ul>
            <p>
              To exercise these rights, contact us at{" "}
              <a
                href="mailto:privacy@toqui.travel"
                className="text-[var(--color-accent)] hover:underline"
              >
                privacy@toqui.travel
              </a>
              .
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              10. Children
            </h2>
            <p>
              Toqui is not intended for anyone under the age of 18. We do not knowingly collect
              information from children.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              11. Changes to This Policy
            </h2>
            <p>
              We may update this policy from time to time. We will notify you of significant changes
              via the application or email. Continued use of Toqui after changes constitutes
              acceptance of the updated policy.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              12. Contact
            </h2>
            <p>
              If you have questions about this policy, contact us at{" "}
              <a
                href="mailto:privacy@toqui.travel"
                className="text-[var(--color-accent)] hover:underline"
              >
                privacy@toqui.travel
              </a>
              .
            </p>
          </section>
        </article>

        <footer className="mt-16 pt-8 border-t border-[var(--color-border)] text-sm text-[var(--color-text-tertiary)]">
          <div className="flex gap-6">
            <Link href="/terms" className="hover:text-[var(--color-accent)] transition-colors">
              Terms of Service
            </Link>
            <Link href="/privacy" className="text-[var(--color-accent)]">
              Privacy Policy
            </Link>
          </div>
        </footer>
      </div>
    </main>
  );
}
