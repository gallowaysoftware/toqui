import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Terms of Service — Toqui",
  description: "Terms and conditions for using the Toqui travel planning service.",
};

export default function TermsOfServicePage() {
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
            Terms of Service
          </h1>
          <p className="text-sm text-[var(--color-text-tertiary)] mb-8">
            Last updated: March 8, 2026
          </p>

          <section className="space-y-4 text-[var(--color-text-secondary)]">
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              1. Acceptance of Terms
            </h2>
            <p>
              By accessing or using Toqui (&quot;the Service&quot;), operated by Galloway Software
              Solutions Inc. (&quot;we,&quot; &quot;us,&quot; or &quot;our&quot;), you agree to be
              bound by these Terms of Service. If you do not agree, do not use the Service.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              2. Eligibility
            </h2>
            <p>
              You must be at least 18 years old to use Toqui. By using the Service, you represent
              that you meet this age requirement.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              3. Description of Service
            </h2>
            <p>
              Toqui is an AI-powered travel planning application that helps you create trips,
              generate itineraries, discover destinations, and find booking recommendations through
              conversational AI. The Service uses artificial intelligence to generate responses and
              suggestions.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              4. AI-Generated Content Disclaimer
            </h2>
            <p>
              Toqui uses AI language models to generate travel recommendations, itineraries, and
              conversational responses. You acknowledge and agree that:
            </p>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                AI-generated content may contain inaccuracies, outdated information, or errors
              </li>
              <li>
                Recommendations are suggestions only and do not constitute professional travel
                advice
              </li>
              <li>
                You are responsible for verifying all information before making travel decisions,
                including prices, availability, visa requirements, health advisories, and safety
                conditions
              </li>
              <li>We are not liable for decisions made based on AI-generated content</li>
            </ul>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              5. Accounts &amp; Authentication
            </h2>
            <p>
              You sign in using Google OAuth. You are responsible for maintaining the security of
              your Google account. We are not liable for unauthorized access resulting from
              compromised third-party credentials.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              6. Acceptable Use
            </h2>
            <p>You agree not to:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Use the Service for any unlawful purpose</li>
              <li>Attempt to bypass usage limits or rate limiting</li>
              <li>
                Use automated systems to access the Service in a manner that exceeds reasonable use
              </li>
              <li>Interfere with or disrupt the Service or its infrastructure</li>
              <li>Reverse-engineer or extract AI model weights or prompts</li>
            </ul>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              7. Affiliate Links &amp; Booking Recommendations
            </h2>
            <p>
              Toqui may display booking recommendations that include affiliate links. When you make
              a purchase through these links, we may earn a commission at no additional cost to you.
              Affiliate relationships do not influence recommendations for Trip Pro users. Free-tier
              recommendations may prioritize affiliate partners.
            </p>
            <p>
              Bookings are completed directly with third-party providers (e.g., Booking.com,
              GetYourGuide). We are not a party to those transactions and are not responsible for
              the services provided by third parties.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              8. Payments &amp; Purchases
            </h2>
            <p>
              Toqui offers free and paid tiers. Paid features (Trip Pro, Annual Pass) are one-time
              purchases — there are no recurring subscriptions. All purchases are final unless
              required otherwise by applicable law. Pricing is displayed at the time of purchase.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              9. Intellectual Property
            </h2>
            <p>
              The Toqui application, including its design, code, and branding, is owned by Galloway
              Software. AI-generated content created for you (itineraries, recommendations, etc.)
              may be used by you for personal purposes. You retain ownership of any content you
              provide to the Service.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              10. Limitation of Liability
            </h2>
            <p>
              To the maximum extent permitted by law, Galloway Software Solutions Inc. is not liable
              for any indirect, incidental, special, consequential, or punitive damages arising from
              your use of the Service. Our total liability for any claim related to the Service is
              limited to the amount you paid us in the 12 months preceding the claim, or $50,
              whichever is greater.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              11. Disclaimer of Warranties
            </h2>
            <p>
              The Service is provided &quot;as is&quot; and &quot;as available&quot; without
              warranties of any kind, express or implied. We do not warrant that the Service will be
              uninterrupted, error-free, or that AI-generated content will be accurate or complete.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              12. Termination
            </h2>
            <p>
              We may suspend or terminate your access to the Service at any time for violation of
              these Terms or for any other reason at our discretion. You may delete your account at
              any time through the application settings.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              13. Changes to Terms
            </h2>
            <p>
              We may update these Terms from time to time. We will notify you of material changes
              via the application or email. Continued use of the Service after changes constitutes
              acceptance of the updated Terms.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              14. Governing Law
            </h2>
            <p>
              These Terms are governed by the laws of Canada. Any disputes will be resolved in the
              courts of British Columbia, Canada.
            </p>

            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              15. Contact
            </h2>
            <p>
              If you have questions about these Terms, contact us at{" "}
              <a
                href="mailto:legal@toqui.travel"
                className="text-[var(--color-accent)] hover:underline"
              >
                legal@toqui.travel
              </a>
              .
            </p>
          </section>
        </article>

        <footer className="mt-16 pt-8 border-t border-[var(--color-border)] text-sm text-[var(--color-text-tertiary)]">
          <div className="flex gap-6">
            <Link href="/terms" className="text-[var(--color-accent)]">
              Terms of Service
            </Link>
            <Link href="/privacy" className="hover:text-[var(--color-accent)] transition-colors">
              Privacy Policy
            </Link>
          </div>
        </footer>
      </div>
    </main>
  );
}
