import { Text, View, StyleSheet, ScrollView } from "react-native";
import { useTheme } from "@/lib/theme";

export default function TermsScreen() {
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
      <Text style={styles.title}>Terms of Service</Text>
      <Text style={styles.lastUpdated}>Last updated: April 1, 2026</Text>

      <Text style={styles.text}>
        These Terms of Service ("Terms") govern your use of Toqui, an
        AI-powered travel planning application operated by Galloway Software
        ("we," "us," or "our"). By creating an account or using the Service,
        you agree to these Terms. If you do not agree, do not use the Service.
      </Text>

      <Text style={styles.sectionTitle}>1. Eligibility</Text>
      <Text style={styles.text}>
        You must be at least 18 years old to use Toqui. By using the Service,
        you represent and warrant that you are at least 18 years of age. We
        reserve the right to terminate accounts that do not meet this
        requirement.
      </Text>

      <Text style={styles.sectionTitle}>2. Account Registration</Text>
      <Text style={styles.text}>
        You must create an account (email + password, or Google OAuth where
        your server has it enabled) to use Toqui. You are responsible for
        maintaining the security of your account credentials and
        for all activity that occurs under your account. You agree to notify us
        immediately at support@toqui.travel if you suspect unauthorized access
        to your account.
      </Text>

      <Text style={styles.sectionTitle}>3. The Service</Text>
      <Text style={styles.text}>
        Toqui provides AI-powered travel planning tools, including an AI chat
        companion, itinerary generation, booking management, and trip sharing.
        The Service is available on web, iOS, and Android platforms.
      </Text>

      <Text style={styles.sectionTitle}>
        4. AI-Generated Content Disclaimer
      </Text>
      <Text style={styles.text}>
        Toqui uses artificial intelligence (the AI provider configured by
        your server operator — e.g. Google Gemini, Anthropic Claude, or a
        self-hosted model) to generate travel suggestions, itineraries,
        recommendations, and other content. You acknowledge and agree that:
      </Text>
      <Bullet>
        AI-generated content is provided for informational and planning purposes
        only and does not constitute professional travel advice
      </Bullet>
      <Bullet>
        AI outputs may contain errors, inaccuracies, or outdated information
        regarding pricing, availability, operating hours, visa requirements, or
        safety conditions
      </Bullet>
      <Bullet>
        You are solely responsible for independently verifying all travel
        information, bookings, and arrangements before relying on them
      </Bullet>
      <Bullet>
        We are not liable for any decisions made or actions taken based on
        AI-generated content
      </Bullet>

      <Text style={styles.sectionTitle}>5. Acceptable Use</Text>
      <Text style={styles.text}>You agree not to use the Service to:</Text>
      <Bullet>
        Violate any applicable law, regulation, or third-party rights
      </Bullet>
      <Bullet>
        Submit false, misleading, or fraudulent information
      </Bullet>
      <Bullet>
        Attempt to gain unauthorized access to the Service, other user accounts,
        or our systems
      </Bullet>
      <Bullet>
        Use automated tools, bots, or scrapers to access the Service without
        our written permission
      </Bullet>
      <Bullet>
        Interfere with or disrupt the integrity or performance of the Service
      </Bullet>
      <Bullet>
        Reverse engineer, decompile, or disassemble any part of the Service
      </Bullet>
      <Bullet>
        Use the AI features to generate harmful, illegal, or abusive content
      </Bullet>
      <Bullet>
        Resell, redistribute, or commercially exploit the Service or its outputs
        without our written consent
      </Bullet>

      <Text style={styles.sectionTitle}>6. Your Content</Text>
      <Text style={styles.text}>
        You retain ownership of the content you submit to Toqui, including trip
        details, chat messages, and booking information. By submitting content,
        you grant us a limited, non-exclusive license to use, store, and process
        that content solely to provide and improve the Service.
      </Text>
      <Text style={styles.text}>
        If you share a trip via a public link, you acknowledge that the trip
        details on that shared page are accessible to anyone with the link.
      </Text>

      <Text style={styles.sectionTitle}>7. Intellectual Property</Text>
      <Text style={styles.text}>
        The Service, including its design, features, code, and branding, is
        owned by Galloway Software and is protected by intellectual property
        laws. You may not copy, modify, distribute, or create derivative works
        from the Service except as expressly permitted by these Terms.
      </Text>
      <Text style={styles.text}>
        AI-generated itineraries and travel content created through your use of
        the Service are provided for your personal use. You may export and share
        these outputs for personal, non-commercial purposes.
      </Text>

      <Text style={styles.sectionTitle}>
        8. Account Suspension and Termination
      </Text>
      <Text style={styles.text}>
        We may suspend or terminate your account at our discretion if you
        violate these Terms, engage in fraudulent activity, or if we reasonably
        believe your use of the Service poses a risk to us or other users. We
        will make reasonable efforts to notify you before or at the time of
        suspension.
      </Text>
      <Text style={styles.text}>
        You may delete your account at any time through the app settings. Upon
        deletion, we will remove your personal data in accordance with our
        Privacy Policy, subject to any legal retention obligations.
      </Text>

      <Text style={styles.sectionTitle}>9. Third-Party Services</Text>
      <Text style={styles.text}>
        The Service integrates with and may link to third-party services,
        including the configured AI provider, optional Google services
        (OAuth, Places API), and various travel booking platforms. We are not
        responsible for the content, accuracy, or practices of these third-party
        services. Your use of third-party services is subject to their
        respective terms and privacy policies.
      </Text>

      <Text style={styles.sectionTitle}>10. Disclaimer of Warranties</Text>
      <Text style={styles.text}>
        THE SERVICE IS PROVIDED "AS IS" AND "AS AVAILABLE" WITHOUT WARRANTIES
        OF ANY KIND, WHETHER EXPRESS, IMPLIED, OR STATUTORY. TO THE FULLEST
        EXTENT PERMITTED BY LAW, WE DISCLAIM ALL WARRANTIES INCLUDING IMPLIED
        WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND
        NON-INFRINGEMENT.
      </Text>
      <Text style={styles.text}>
        Without limiting the foregoing, we do not warrant that the Service will
        be uninterrupted, error-free, or secure, that any AI-generated content
        will be accurate or reliable, or that the Service will meet your
        specific travel planning needs.
      </Text>

      <Text style={styles.sectionTitle}>11. Limitation of Liability</Text>
      <Text style={styles.text}>
        TO THE MAXIMUM EXTENT PERMITTED BY LAW, GALLOWAY SOFTWARE AND ITS
        OFFICERS, DIRECTORS, EMPLOYEES, AND AGENTS SHALL NOT BE LIABLE FOR ANY
        INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL, OR PUNITIVE DAMAGES, OR
        ANY LOSS OF PROFITS, DATA, OR GOODWILL, ARISING OUT OF OR RELATED TO
        YOUR USE OF THE SERVICE.
      </Text>
      <Text style={styles.text}>
        OUR TOTAL AGGREGATE LIABILITY FOR ALL CLAIMS ARISING OUT OF OR RELATED
        TO THESE TERMS OR THE SERVICE SHALL NOT EXCEED THE AMOUNT YOU HAVE PAID
        US IN THE TWELVE (12) MONTHS PRECEDING THE CLAIM, OR $100, WHICHEVER IS
        GREATER.
      </Text>
      <Text style={styles.text}>
        This limitation applies regardless of the theory of liability (contract,
        tort, strict liability, or otherwise) and even if we have been advised
        of the possibility of such damages.
      </Text>

      <Text style={styles.sectionTitle}>12. Indemnification</Text>
      <Text style={styles.text}>
        You agree to indemnify and hold harmless Galloway Software from any
        claims, damages, losses, or expenses (including reasonable legal fees)
        arising out of your use of the Service, your violation of these Terms,
        or your violation of any third-party rights.
      </Text>

      <Text style={styles.sectionTitle}>13. Governing Law and Disputes</Text>
      <Text style={styles.text}>
        These Terms are governed by the laws of the Province of Prince Edward
        Island and the federal laws of Canada applicable therein, without
        regard to conflict of law principles. Any disputes arising from these
        Terms or the Service shall be resolved in the courts of Prince Edward
        Island, Canada. You consent to the personal jurisdiction of such
        courts.
      </Text>

      <Text style={styles.sectionTitle}>14. Changes to These Terms</Text>
      <Text style={styles.text}>
        We may update these Terms from time to time. We will notify you of
        material changes by posting the updated Terms within the app and
        updating the "Last updated" date. Your continued use of the Service
        after changes are posted constitutes acceptance of the updated Terms.
        If you do not agree to the updated Terms, you must stop using the
        Service.
      </Text>

      <Text style={styles.sectionTitle}>15. General Provisions</Text>
      <Bullet>
        Severability: If any provision of these Terms is found to be
        unenforceable, the remaining provisions remain in full effect
      </Bullet>
      <Bullet>
        Waiver: Our failure to enforce any right or provision does not
        constitute a waiver of that right or provision
      </Bullet>
      <Bullet>
        Entire agreement: These Terms, together with our Privacy Policy,
        constitute the entire agreement between you and Galloway Software
        regarding the Service
      </Bullet>
      <Bullet>
        Assignment: You may not assign your rights under these Terms without our
        written consent; we may assign our rights without restriction
      </Bullet>

      <Text style={styles.sectionTitle}>16. Contact Us</Text>
      <Text style={styles.text}>
        If you have questions about these Terms, contact us at:
      </Text>
      <Text style={styles.text}>
        Galloway Software{"\n"}
        Email: support@toqui.travel{"\n"}
        Website: https://toqui.travel
      </Text>
    </ScrollView>
  );
}
