import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Privacy Policy",
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
          &larr; Back to Toqui
        </Link>

        <article className="prose prose-neutral dark:prose-invert max-w-none">
          <h1 className="text-3xl font-bold tracking-tight text-[var(--color-text-primary)] mb-2">
            Privacy Policy
          </h1>
          <p className="text-sm text-[var(--color-text-tertiary)] mb-8">
            Last updated: March 2026
          </p>

          <section className="space-y-4 text-[var(--color-text-secondary)]">
            <p>
              Galloway Software Solutions Inc., operating as Toqui (&quot;we,&quot; &quot;us,&quot;
              or &quot;our&quot;), provides the Toqui AI travel companion application and website.
              This Privacy Policy explains how we collect, use, disclose, and safeguard your
              information when you use our service.
            </p>
            <p>
              By using Toqui, you consent to the data practices described in this policy. If you do
              not agree, please do not use the service.
            </p>

            {/* 1. Information We Collect */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              1. Information We Collect
            </h2>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              1.1 Account Information
            </h3>
            <p>When you create an account via Google OAuth, we collect:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Email address</li>
              <li>Display name</li>
              <li>Profile photo URL (from Google)</li>
              <li>Google account identifier</li>
            </ul>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              1.2 Trip Data
            </h3>
            <p>Information you provide about your trips:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Trip titles, descriptions, and dates</li>
              <li>Itinerary items (places, times, notes)</li>
              <li>Booking confirmations you upload or forward</li>
              <li>Trip themes and destination information</li>
            </ul>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              1.3 Chat Conversations
            </h3>
            <p>
              Messages exchanged with Toqui and expert guides are stored to maintain conversation
              context and improve your experience. Chat data is associated with your trip and subject
              to the data retention schedule described in Section 7.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              1.4 Location Data
            </h3>
            <p className="font-semibold">
              Your real-time location is NEVER stored permanently.
            </p>
            <p>
              In Companion Mode, you may choose to share your current location to receive nearby
              recommendations. This location data is:
            </p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Ephemeral and request-scoped only</li>
              <li>Used only for the duration of that single request</li>
              <li>
                Passed to the AI as temporary context to generate relevant recommendations
              </li>
              <li>Immediately discarded after the response is generated</li>
              <li>Never written to any database, log, or persistent storage</li>
              <li>Never shared with third parties</li>
            </ul>
            <p>
              Itinerary items may include location coordinates for places you explicitly add to your
              trip (e.g., a restaurant address). These are places you chose, not your personal
              location.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              1.5 Booking Details
            </h3>
            <p>
              When you paste or forward booking confirmations (flights, hotels, activities), Toqui
              extracts structured data such as dates, times, locations, and confirmation numbers.
              This data is stored as part of your trip and subject to the retention schedule in
              Section 7.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              1.6 Usage Data
            </h3>
            <p>
              We collect basic usage metrics such as message counts and feature usage to improve the
              service. We do not use third-party analytics trackers in the application.
            </p>

            {/* 2. How We Use Your Information */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              2. How We Use Your Information
            </h2>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                <strong>Provide the service:</strong> Plan trips, generate itineraries, manage
                bookings, and deliver personalized travel recommendations.
              </li>
              <li>
                <strong>AI processing:</strong> Your messages and trip context are sent to AI
                language model providers to generate responses. We do not use your data to train AI
                models.
              </li>
              <li>
                <strong>Persona matching:</strong> Your trip themes and destination are used to match
                you with relevant expert guide personas.
              </li>
              <li>
                <strong>Recommendations:</strong> We may provide travel recommendations that include
                links to affiliate partners. See Section 6 for details.
              </li>
              <li>
                <strong>Service improvement:</strong> Aggregated, anonymized usage patterns may be
                used to improve the service. Individual conversations are not reviewed by humans
                unless you report an issue.
              </li>
              <li>
                <strong>Communications:</strong> We may use your email address for service-related
                notifications, including material changes to this policy.
              </li>
            </ul>

            {/* 3. Legal Basis for Processing */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              3. Legal Basis for Processing
            </h2>
            <p>
              Under GDPR Article 6, we process your personal data on the following legal bases:
            </p>
            <div className="overflow-x-auto my-4">
              <table className="w-full text-sm border border-[var(--color-border)] rounded-lg">
                <thead>
                  <tr className="bg-[var(--color-surface-secondary)]">
                    <th className="text-left px-4 py-2 font-semibold text-[var(--color-text-primary)]">
                      Processing Activity
                    </th>
                    <th className="text-left px-4 py-2 font-semibold text-[var(--color-text-primary)]">
                      Legal Basis
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">Account creation and authentication</td>
                    <td className="px-4 py-2">
                      <strong>Contractual necessity</strong> (Art. 6(1)(b)) &mdash; required to
                      provide the service
                    </td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">Trip planning and itinerary management</td>
                    <td className="px-4 py-2">
                      <strong>Contractual necessity</strong> (Art. 6(1)(b)) &mdash; core service
                      functionality
                    </td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">AI chat processing</td>
                    <td className="px-4 py-2">
                      <strong>Contractual necessity</strong> (Art. 6(1)(b)) &mdash; required to
                      generate travel recommendations
                    </td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">Booking data storage</td>
                    <td className="px-4 py-2">
                      <strong>Contractual necessity</strong> (Art. 6(1)(b)) &mdash; to manage your
                      travel bookings
                    </td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">Real-time location (Companion Mode)</td>
                    <td className="px-4 py-2">
                      <strong>Consent</strong> (Art. 6(1)(a)) &mdash; opt-in only, never stored
                    </td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">Service improvement (aggregated analytics)</td>
                    <td className="px-4 py-2">
                      <strong>Legitimate interest</strong> (Art. 6(1)(f)) &mdash; improving service
                      quality
                    </td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">Service-related communications</td>
                    <td className="px-4 py-2">
                      <strong>Legitimate interest</strong> (Art. 6(1)(f)) &mdash; notifying you of
                      material changes
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p>
              Where processing is based on consent, you may withdraw your consent at any time.
              Withdrawing consent does not affect the lawfulness of processing performed prior to
              withdrawal. Where processing is based on legitimate interest, you have the right to
              object (see Section 8).
            </p>

            {/* 4. AI and Data Processing */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              4. AI and Data Processing
            </h2>
            <p>
              Toqui uses third-party AI providers to generate travel recommendations and
              conversational responses. You should be aware of the following:
            </p>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                Chat messages and trip context are sent to AI providers (Anthropic Claude, Google
                Gemini) for processing.
              </li>
              <li>
                We use API-only access. Per our providers&apos; API terms, your data is{" "}
                <strong>not used to train their models</strong>.
              </li>
              <li>
                We do not send your email address, name, or account information to AI providers
                &mdash; only trip context and message content.
              </li>
              <li>
                AI providers process data under their respective data processing agreements and do
                not retain conversation data beyond the API request.
              </li>
              <li>
                <strong>AI-generated content may contain errors.</strong> Recommendations,
                itineraries, and other AI outputs are provided for informational purposes. Always
                verify critical details independently.
              </li>
            </ul>

            {/* 5. Third-Party Services */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              5. Third-Party Services
            </h2>
            <p>We use the following third-party services to operate Toqui:</p>
            <div className="overflow-x-auto my-4">
              <table className="w-full text-sm border border-[var(--color-border)] rounded-lg">
                <thead>
                  <tr className="bg-[var(--color-surface-secondary)]">
                    <th className="text-left px-4 py-2 font-semibold text-[var(--color-text-primary)]">
                      Service
                    </th>
                    <th className="text-left px-4 py-2 font-semibold text-[var(--color-text-primary)]">
                      Purpose
                    </th>
                    <th className="text-left px-4 py-2 font-semibold text-[var(--color-text-primary)]">
                      Data Shared
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">
                      <a
                        href="https://www.anthropic.com/privacy"
                        className="text-[var(--color-accent)] hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        Anthropic
                      </a>{" "}
                      (Claude)
                    </td>
                    <td className="px-4 py-2">AI chat processing</td>
                    <td className="px-4 py-2">Chat messages, trip context</td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">
                      <a
                        href="https://policies.google.com/privacy"
                        className="text-[var(--color-accent)] hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        Google
                      </a>{" "}
                      (Gemini)
                    </td>
                    <td className="px-4 py-2">AI chat processing</td>
                    <td className="px-4 py-2">Chat messages, trip context</td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">
                      <a
                        href="https://policies.google.com/privacy"
                        className="text-[var(--color-accent)] hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        Google
                      </a>
                    </td>
                    <td className="px-4 py-2">Authentication (OAuth)</td>
                    <td className="px-4 py-2">Standard OAuth flow data</td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">
                      <a
                        href="https://stripe.com/privacy"
                        className="text-[var(--color-accent)] hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        Stripe
                      </a>
                    </td>
                    <td className="px-4 py-2">Payment processing</td>
                    <td className="px-4 py-2">Payment details (handled by Stripe directly)</td>
                  </tr>
                  <tr className="border-t border-[var(--color-border)]">
                    <td className="px-4 py-2">
                      <a
                        href="https://sendgrid.com/policies/privacy/"
                        className="text-[var(--color-accent)] hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        SendGrid
                      </a>
                    </td>
                    <td className="px-4 py-2">Transactional email</td>
                    <td className="px-4 py-2">Email address, message content</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p>
              These sub-processors process data under their respective data processing agreements
              (DPAs) and privacy policies. All infrastructure services (hosting, database, storage)
              are provided by Google Cloud Platform under Google&apos;s{" "}
              <a
                href="https://cloud.google.com/terms/data-processing-addendum"
                className="text-[var(--color-accent)] hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                Cloud Data Processing Addendum
              </a>
              . If you require a copy of our sub-processor list or DPA for your organization,
              contact us at{" "}
              <a
                href="mailto:privacy@toqui.travel"
                className="text-[var(--color-accent)] hover:underline"
              >
                privacy@toqui.travel
              </a>
              .
            </p>

            {/* 6. Data Sharing */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              6. Data Sharing
            </h2>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              6.1 Affiliate Partners
            </h3>
            <p>
              Toqui may include links to third-party travel services. When you follow these links,
              the partner may collect data according to their own privacy policies. Our affiliate
              partners include:
            </p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Booking.com (accommodation)</li>
              <li>Skyscanner / Kayak (flights)</li>
              <li>GetYourGuide / Viator (tours and activities)</li>
              <li>TheFork / OpenTable (restaurant reservations)</li>
            </ul>
            <p>
              We do not share your personal data with affiliate partners. When you click an affiliate
              link, only standard web referral data (click-through data) is transmitted. No personal
              information is sent.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              6.2 What We Do Not Do
            </h3>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                We do <strong>not</strong> sell your personal data to third parties.
              </li>
              <li>
                We do <strong>not</strong> share your personal information with advertisers.
              </li>
              <li>
                We do <strong>not</strong> provide your data to data brokers.
              </li>
            </ul>

            {/* 7. Data Retention */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              7. Data Retention
            </h2>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              7.1 Active Trips
            </h3>
            <p>
              All trip data and chat history is retained while a trip is in planning or active
              status.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              7.2 Completed Trips
            </h3>
            <p>
              When a trip is marked as completed, a 90-day retention period begins for chat messages.
              After this period:
            </p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Chat conversations are permanently deleted</li>
              <li>Itinerary and booking data are retained for your reference</li>
              <li>The trip enters archive mode</li>
            </ul>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              7.3 Deleted Trips
            </h3>
            <p>
              When you delete a trip, all associated data is permanently removed: itinerary items,
              bookings, chat history, and theme associations. This deletion is immediate and
              irreversible.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              7.4 Account Deletion
            </h3>
            <p>
              When you delete your account, all data is permanently removed within 30 days, including
              all trips, conversations, and profile information.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              7.5 Server Logs
            </h3>
            <p>
              Server logs (request metadata, error logs, audit events) are retained for 90 days for
              security monitoring and debugging purposes. Logs do not contain full email addresses
              &mdash; they are masked in audit records. After 90 days, logs are automatically purged.
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              7.6 Waitlist Data
            </h3>
            <p>
              If you join the waitlist, your email is retained until you are invited and create an
              account, at which point it becomes part of your account data. You may request removal
              from the waitlist at any time by contacting us.
            </p>

            {/* 8. Your Rights */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              8. Your Rights
            </h2>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              8.1 Under GDPR (European Economic Area)
            </h3>
            <p>If you are in the EEA, you have the right to:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                <strong>Access</strong> (Article 15) &mdash; Request a copy of all personal data we
                hold about you
              </li>
              <li>
                <strong>Rectification</strong> (Article 16) &mdash; Request correction of inaccurate
                data
              </li>
              <li>
                <strong>Erasure</strong> (Article 17) &mdash; Request deletion of your account and
                all associated data within 30 days
              </li>
              <li>
                <strong>Data portability</strong> (Article 20) &mdash; Receive your data in a
                structured, machine-readable format (JSON export)
              </li>
              <li>
                <strong>Restrict processing</strong> (Article 18) &mdash; Request that we limit how
                we use your data
              </li>
              <li>
                <strong>Object</strong> (Article 21) &mdash; Object to processing of your data for
                specific purposes
              </li>
              <li>
                <strong>Withdraw consent</strong> &mdash; Withdraw consent at any time where
                processing is based on consent
              </li>
            </ul>
            <p>We will respond to all GDPR requests within 30 days.</p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              8.2 Under PIPEDA (Canada)
            </h3>
            <p>As a Canadian service, we comply with PIPEDA. You have the right to:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                <strong>Access</strong> &mdash; Request access to your personal information
              </li>
              <li>
                <strong>Correction</strong> &mdash; Request correction of inaccurate information
              </li>
              <li>
                <strong>Withdraw consent</strong> &mdash; Withdraw consent for non-essential data
                processing
              </li>
              <li>
                <strong>Complain</strong> &mdash; File a complaint with the Office of the Privacy
                Commissioner of Canada
              </li>
            </ul>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              8.3 Under CCPA (California)
            </h3>
            <p>If you are a California resident, you have the right to:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                <strong>Know</strong> &mdash; Request what personal information we collect, use, and
                disclose
              </li>
              <li>
                <strong>Delete</strong> &mdash; Request deletion of your personal information
              </li>
              <li>
                <strong>Opt-out</strong> &mdash; Opt out of the sale of personal information (we do
                not sell your data)
              </li>
              <li>
                <strong>Non-discrimination</strong> &mdash; Not be discriminated against for
                exercising your rights
              </li>
            </ul>
            <p>We do not sell personal information to third parties.</p>

            {/* 9. How to Exercise Your Rights */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              9. How to Exercise Your Rights
            </h2>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              Delete Your Account
            </h3>
            <p>
              You can delete your account and all associated data at any time from your account
              settings in the app. This action deletes:
            </p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Your user profile</li>
              <li>All trips, itineraries, and bookings</li>
              <li>All chat conversations across all trips</li>
              <li>All theme and persona associations</li>
            </ul>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              Export Your Data
            </h3>
            <p>
              You can request a full export of your data from your account settings. Your data will
              be prepared as a JSON file and made available for download within 24 hours, in
              compliance with GDPR Article 20 (data portability).
            </p>

            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mt-4 mb-2">
              Contact Us
            </h3>
            <p>
              For any privacy-related requests or questions, contact us at{" "}
              <a
                href="mailto:privacy@toqui.travel"
                className="text-[var(--color-accent)] hover:underline"
              >
                privacy@toqui.travel
              </a>
              .
            </p>

            {/* 10. Data Security */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              10. Data Security
            </h2>
            <p>
              We implement appropriate technical and organizational measures to protect your personal
              data, including:
            </p>
            <ul className="list-disc pl-6 space-y-1">
              <li>Encryption in transit (TLS) and at rest</li>
              <li>Authentication via OAuth 2.0 (no passwords stored)</li>
              <li>HttpOnly authentication cookies (not accessible to JavaScript)</li>
              <li>Role-based access controls for internal systems</li>
              <li>Regular security reviews</li>
            </ul>

            {/* 11. Cookies & Local Storage */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              11. Cookies &amp; Local Storage
            </h2>
            <p>The Toqui application uses minimal cookies:</p>
            <ul className="list-disc pl-6 space-y-1">
              <li>
                <strong>Authentication tokens</strong> &mdash; HttpOnly session cookies required for
                the service to function. These are not tracking cookies.
              </li>
            </ul>
            <p>
              We do not use advertising cookies or third-party tracking cookies in the application.
              We also use your browser&apos;s local storage to remember preferences (such as
              dark/light mode) and age verification status.
            </p>

            {/* 12. Age Requirement */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              12. Age Requirement
            </h2>
            <p>
              Toqui is not intended for use by anyone under the age of 18. We do not knowingly
              collect personal information from individuals under 18. If we become aware that we have
              collected data from someone under 18, we will delete it promptly.
            </p>

            {/* 13. International Data Transfers */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              13. International Data Transfers
            </h2>
            <p>
              Your data may be processed in Canada, the United States, and other jurisdictions where
              our service providers operate. We ensure appropriate safeguards are in place for
              international data transfers in compliance with applicable data protection laws.
            </p>

            {/* 14. Changes to This Policy */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              14. Changes to This Policy
            </h2>
            <p>
              We may update this Privacy Policy from time to time. For material changes, we will
              notify you via email at the address associated with your account and update the
              effective date on this page. Continued use of the service after changes constitutes
              acceptance of the updated policy.
            </p>

            {/* 15. Governing Law */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              15. Governing Law
            </h2>
            <p>
              This Privacy Policy is governed by the laws of the Province of British Columbia and the
              federal laws of Canada applicable therein. Any disputes relating to this policy will be
              resolved in the courts of British Columbia, Canada.
            </p>

            {/* 16. Contact */}
            <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mt-8 mb-3">
              16. Contact
            </h2>
            <p>If you have questions about this Privacy Policy or our data practices:</p>
            <p className="mt-2">
              Galloway Software Solutions Inc.
              <br />
              British Columbia, Canada
              <br />
              Email:{" "}
              <a
                href="mailto:privacy@toqui.travel"
                className="text-[var(--color-accent)] hover:underline"
              >
                privacy@toqui.travel
              </a>
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
