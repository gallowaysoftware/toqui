import { Text, View, StyleSheet, ScrollView } from "react-native";
import { useTheme } from "@/lib/theme";

export default function PrivacyScreen() {
  const { colors } = useTheme();

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: colors.surface },
    content: { padding: 24, paddingBottom: 48 },
    title: {
      fontSize: 28,
      fontWeight: "bold",
      marginBottom: 8,
      color: colors.textPrimary,
    },
    lastUpdated: {
      fontSize: 14,
      color: colors.textSecondary,
      marginBottom: 24,
    },
    sectionTitle: {
      fontSize: 20,
      fontWeight: "700",
      marginTop: 28,
      marginBottom: 12,
      color: colors.textPrimary,
    },
    subsectionTitle: {
      fontSize: 17,
      fontWeight: "600",
      marginTop: 16,
      marginBottom: 8,
      color: colors.textPrimary,
    },
    text: {
      fontSize: 16,
      color: colors.textSecondary,
      lineHeight: 24,
      marginBottom: 12,
    },
    bulletContainer: {
      flexDirection: "row",
      paddingLeft: 16,
      marginBottom: 6,
    },
    bullet: {
      fontSize: 16,
      color: colors.textSecondary,
      lineHeight: 24,
      marginRight: 8,
    },
    bulletText: {
      fontSize: 16,
      color: colors.textSecondary,
      lineHeight: 24,
      flex: 1,
    },
  });

  const Bullet = ({ children }: { children: string }) => (
    <View style={styles.bulletContainer}>
      <Text style={styles.bullet}>{"\u2022"}</Text>
      <Text style={styles.bulletText}>{children}</Text>
    </View>
  );

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.title}>Privacy Policy</Text>
      <Text style={styles.lastUpdated}>Last updated: April 2, 2026</Text>

      <Text style={styles.text}>
        Galloway Software ("we," "us," or "our") operates Toqui, an AI-powered
        travel planning application available on web, iOS, and Android. This
        Privacy Policy explains how we collect, use, share, and protect your
        personal information when you use Toqui (the "Service").
      </Text>
      <Text style={styles.text}>
        By using Toqui, you agree to the collection and use of information as
        described in this policy. If you do not agree, please do not use the
        Service.
      </Text>

      <Text style={styles.sectionTitle}>1. Information We Collect</Text>

      <Text style={styles.subsectionTitle}>Account Information</Text>
      <Text style={styles.text}>
        When you create an account, we collect information from your Google
        account through Google OAuth, including:
      </Text>
      <Bullet>Name</Bullet>
      <Bullet>Email address</Bullet>
      <Bullet>Google profile picture</Bullet>
      <Bullet>Age verification status (we require users to be 18 or older)</Bullet>

      <Text style={styles.subsectionTitle}>Trip and Chat Data</Text>
      <Text style={styles.text}>
        When you use Toqui to plan trips, we collect:
      </Text>
      <Bullet>Trip details (destinations, dates, preferences)</Bullet>
      <Bullet>Chat messages you send to the AI travel companion</Bullet>
      <Bullet>AI-generated responses and itineraries</Bullet>
      <Bullet>Bookings and reservation details you add or forward to us</Bullet>
      <Bullet>Exported itineraries (PDF and calendar files)</Bullet>

      <Text style={styles.subsectionTitle}>Payment Information</Text>
      <Text style={styles.text}>
        Payments for Trip Pro upgrades are processed by Helcim, our third-party
        payment processor. We do not directly store your credit card number or
        full payment details. We receive a transaction confirmation and associate
        the purchase with your account. Helcim's privacy practices are governed
        by their own privacy policy.
      </Text>

      <Text style={styles.subsectionTitle}>Location Data</Text>
      <Text style={styles.text}>
        When you use the travel companion feature, we may collect your
        approximate location to provide location-relevant travel suggestions.
        You can control location access through your device settings.
      </Text>

      <Text style={styles.subsectionTitle}>Email Forwarding</Text>
      <Text style={styles.text}>
        If you use our booking email forwarding feature (available with Trip
        Pro), we process inbound emails sent to your unique forwarding address
        to extract booking details. These emails are processed by SendGrid and
        stored only long enough to extract relevant booking information.
      </Text>

      <Text style={styles.subsectionTitle}>Usage and Device Information</Text>
      <Text style={styles.text}>
        We automatically collect standard technical information including device
        type, operating system, app version, and general usage patterns to
        maintain and improve the Service.
      </Text>

      <Text style={styles.subsectionTitle}>Analytics</Text>
      <Text style={styles.text}>
        We use PostHog for product analytics, hosted in the European Union. We
        collect information about how you use the app — which features you use,
        how often you visit, and general interaction patterns — to improve the
        Service.
      </Text>
      <Text style={styles.text}>
        Here is what we do NOT track through analytics:
      </Text>
      <Bullet>
        Your trip content (destinations, dates, chat messages, booking details)
      </Bullet>
      <Bullet>Your activity across other apps or websites</Bullet>
      <Text style={styles.text}>
        Your user ID is pseudonymized (hashed) before being sent to our
        analytics provider. We do not use cookies for analytics. You can opt out
        of analytics at any time in your account settings, or by contacting us
        at privacy@toqui.travel.
      </Text>

      <Text style={styles.sectionTitle}>2. How We Use Your Information</Text>
      <Text style={styles.text}>We use your information to:</Text>
      <Bullet>Provide, operate, and improve the Service</Bullet>
      <Bullet>
        Generate personalized travel plans and recommendations through our AI
        system
      </Bullet>
      <Bullet>Process payments and manage your account</Bullet>
      <Bullet>
        Send transactional emails (account verification, booking confirmations,
        trip-related notifications)
      </Bullet>
      <Bullet>
        Parse forwarded booking emails to populate your trip details
      </Bullet>
      <Bullet>Manage the referral program</Bullet>
      <Bullet>Detect and prevent fraud or abuse</Bullet>
      <Bullet>Comply with legal obligations</Bullet>

      <Text style={styles.sectionTitle}>
        3. AI Processing and Third-Party AI Services
      </Text>
      <Text style={styles.text}>
        Toqui uses third-party AI models from Anthropic (Claude) and Google
        (Gemini) to power the travel planning features. When you interact with
        the AI companion, your messages and relevant trip context are sent to
        these providers for processing. These providers process data according
        to their own privacy policies and data processing agreements.
      </Text>
      <Text style={styles.text}>
        We also use Google Places API and Google Custom Search API to provide
        location information and search results relevant to your trip planning.
      </Text>
      <Text style={styles.text}>
        AI-generated content (itineraries, suggestions, recommendations) is
        created algorithmically and may not always be accurate. We do not
        guarantee the accuracy of AI outputs.
      </Text>

      <Text style={styles.sectionTitle}>
        4. How We Share Your Information
      </Text>
      <Text style={styles.text}>
        We do not sell your personal information. We share information only in
        these circumstances:
      </Text>
      <Bullet>
        Service providers: Helcim (payments), SendGrid (email forwarding),
        Resend (transactional emails), Google Cloud Platform (hosting and
        storage), Anthropic and Google (AI processing), PostHog (analytics,
        EU-hosted)
      </Bullet>
      <Bullet>
        Shared trips: If you share a trip via a public link, the trip details
        visible on that page are accessible to anyone with the link
      </Bullet>
      <Bullet>
        Legal requirements: We may disclose information if required by law,
        regulation, legal process, or governmental request
      </Bullet>
      <Bullet>
        Business transfers: In connection with a merger, acquisition, or sale of
        assets, your information may be transferred as a business asset
      </Bullet>

      <Text style={styles.sectionTitle}>5. Data Storage and Security</Text>
      <Text style={styles.text}>
        Your data is stored on Google Cloud Platform infrastructure located in
        the United States. Account data and trip information are stored in
        PostgreSQL databases. Chat history is stored in Google Firestore.
      </Text>
      <Text style={styles.text}>
        Authentication tokens are stored securely on your device: in the
        system keychain (iOS) or keystore (Android) on native platforms, and
        in session storage on web. We use encrypted connections (TLS) for all
        data in transit. We do not use cookies for authentication.
      </Text>
      <Text style={styles.text}>
        While we implement industry-standard security measures, no method of
        transmission or storage is 100% secure. We cannot guarantee absolute
        security.
      </Text>

      <Text style={styles.sectionTitle}>6. Data Retention</Text>
      <Text style={styles.text}>
        We retain your data for as long as your account is active. Completed
        trips are archived after 90 days of completion but remain accessible in
        your account. If you delete your account, we will delete your personal
        data in accordance with our obligations, subject to any legal retention
        requirements.
      </Text>

      <Text style={styles.sectionTitle}>7. Your Rights</Text>
      <Text style={styles.text}>
        You have the following rights regarding your personal data:
      </Text>
      <Bullet>
        Access: You can request a copy of the personal data we hold about you
      </Bullet>
      <Bullet>
        Deletion: You can request deletion of your account and associated data
        (GDPR Article 17)
      </Bullet>
      <Bullet>
        Portability: You can request an export of your data in a portable format
        (GDPR Article 20)
      </Bullet>
      <Bullet>
        Correction: You can request correction of inaccurate personal data
      </Bullet>
      <Bullet>
        Objection: You can object to certain processing of your data
      </Bullet>
      <Text style={styles.text}>
        To exercise any of these rights, contact us at privacy@toqui.travel. We
        will respond to requests within 30 days.
      </Text>

      <Text style={styles.sectionTitle}>8. Children's Privacy</Text>
      <Text style={styles.text}>
        Toqui is not intended for users under the age of 18. We enforce an age
        gate that requires users to verify they are 18 or older before accessing
        the Service. We do not knowingly collect personal information from
        anyone under 18. If we learn that we have collected personal data from a
        user under 18, we will delete that information promptly. If you believe
        a child under 18 has provided us with personal information, please
        contact us at privacy@toqui.travel.
      </Text>

      <Text style={styles.sectionTitle}>
        9. Cookies and Tracking Technologies
      </Text>
      <Text style={styles.text}>
        Toqui does not use cookies for authentication or tracking. On the web
        version, authentication tokens are stored in browser local storage. Our
        analytics provider (PostHog) does not use cookies. We do not use
        third-party tracking cookies or advertising pixels.
      </Text>

      <Text style={styles.sectionTitle}>
        10. International Data Transfers
      </Text>
      <Text style={styles.text}>
        Your data is processed and stored in the United States. If you are
        located outside the United States, your information will be transferred
        to and processed in the United States. By using the Service, you consent
        to this transfer. We take steps to ensure your data is treated securely
        and in accordance with this Privacy Policy regardless of where it is
        processed.
      </Text>

      <Text style={styles.sectionTitle}>
        11. California Privacy Rights (CCPA)
      </Text>
      <Text style={styles.text}>
        If you are a California resident, you have additional rights under the
        California Consumer Privacy Act, including the right to know what
        personal information we collect, the right to request deletion, and the
        right to opt out of the sale of personal information. We do not sell
        personal information. To exercise your California privacy rights,
        contact us at privacy@toqui.travel.
      </Text>

      <Text style={styles.sectionTitle}>12. Changes to This Policy</Text>
      <Text style={styles.text}>
        We may update this Privacy Policy from time to time. We will notify you
        of material changes by posting the updated policy within the app and
        updating the "Last updated" date. Your continued use of the Service
        after changes are posted constitutes acceptance of the updated policy.
      </Text>

      <Text style={styles.sectionTitle}>13. Contact Us</Text>
      <Text style={styles.text}>
        If you have questions or concerns about this Privacy Policy or our data
        practices, contact us at:
      </Text>
      <Text style={styles.text}>
        Galloway Software{"\n"}
        Email: privacy@toqui.travel{"\n"}
        Website: https://toqui.travel
      </Text>
    </ScrollView>
  );
}
