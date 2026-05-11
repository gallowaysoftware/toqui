package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	gcstorage "cloud.google.com/go/storage"
	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/google/uuid"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/auth/apple"
	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/chat"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/config"
	"github.com/gallowaysoftware/toqui-backend/internal/csrf"
	"github.com/gallowaysoftware/toqui-backend/internal/db"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/email"
	"github.com/gallowaysoftware/toqui-backend/internal/exportstorage"
	"github.com/gallowaysoftware/toqui-backend/internal/handlers"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
	"github.com/gallowaysoftware/toqui-backend/internal/location"
	"github.com/gallowaysoftware/toqui-backend/internal/middleware"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
	"github.com/gallowaysoftware/toqui-backend/internal/requestid"
	"github.com/gallowaysoftware/toqui-backend/internal/subscription"
	"github.com/gallowaysoftware/toqui-backend/internal/telemetry"
	"github.com/gallowaysoftware/toqui-backend/internal/theme"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
	"github.com/gallowaysoftware/toqui-backend/internal/usage"
	"github.com/gallowaysoftware/toqui-backend/internal/validate"

	toquiv1connect "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1/toquiv1connect"
)

func main() {
	startTime := time.Now()
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	// OpenTelemetry tracing + metrics — non-fatal, server continues without telemetry if init fails.
	// Resolves gcsm:// in OTEL_EXPORTER_OTLP_HEADERS via GCP Secret Manager.
	otelShutdown, otelErr := telemetry.Init(ctx, "toqui-backend", cfg.FirestoreProjectID)
	if otelErr != nil {
		slog.Error("failed to initialize OpenTelemetry, continuing without telemetry", "error", otelErr)
	}
	if otelShutdown != nil {
		defer func() {
			if err := otelShutdown(context.Background()); err != nil {
				slog.Error("OpenTelemetry shutdown error", "error", err)
			}
		}()
	}

	// OpenTelemetry metrics instruments — safe no-ops when exporter is not configured.
	otelMetrics, metricsErr := telemetry.NewMetrics()
	if metricsErr != nil {
		slog.Error("failed to create OpenTelemetry metrics, continuing without metrics", "error", metricsErr)
	}

	// Use JSON structured logging in non-local environments for Cloud Logging.
	// Cloud Logging automatically parses JSON from stdout and indexes severity,
	// message, and structured fields for querying and alerting.
	if cfg.TargetEnv != "local" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})))
	}

	// Analytics (PostHog) — no-op client when API key is empty
	posthogClient := analytics.NewClient(cfg.PostHogAPIKey)
	if posthogClient.Enabled() {
		slog.Info("PostHog analytics enabled", "endpoint", "eu.i.posthog.com")
	}

	// In-process health AlertChecker — emits Cloud Logging warnings when
	// chat/signup traffic stalls (early-detection of upstream breakage
	// where the API stays up but no traffic flows). The goroutine lives
	// for the whole server lifetime; stops on ctx cancel.
	alertChecker := analytics.NewAlertChecker()
	alertChecker.StartPeriodicCheck(ctx, 5*time.Minute)

	// Database
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Firestore
	firestoreClient, err := newFirestoreClient(ctx, cfg)
	if err != nil {
		slog.Error("connect to firestore failed", "error", err)
		os.Exit(1)
	}
	defer firestoreClient.Close()

	// Auth
	authSvc := auth.NewService(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleRedirectURI,
		cfg.JWTSecret,
	)

	// Apple Sign-In: enabled only when all four credentials are present.
	// Until Apple Developer enrollment completes, these are blank in every
	// environment and AppleLogin returns Unimplemented.
	if apple.IsConfigured(cfg.AppleTeamID, cfg.AppleServicesID, cfg.AppleKeyID, []byte(cfg.ApplePrivateKey)) {
		appleClient, err := apple.NewClient(apple.Config{
			TeamID:     cfg.AppleTeamID,
			ServicesID: cfg.AppleServicesID,
			KeyID:      cfg.AppleKeyID,
			PrivateKey: []byte(cfg.ApplePrivateKey),
		})
		if err != nil {
			slog.Error("apple sign-in client initialization failed — AppleLogin will be unimplemented",
				"error", err)
		} else {
			authSvc.WithAppleClient(appleClient)
			slog.Info("apple sign-in configured", "services_id", cfg.AppleServicesID, "team_id", cfg.AppleTeamID)
		}
	} else {
		slog.Warn("apple sign-in not configured (waiting on Apple Developer enrollment) — AppleLogin returns Unimplemented")
	}

	// Allow frontend origins to use their own /auth/callback as redirect URI.
	// This is needed because the SPA sends the auth code to GoogleLogin with
	// the frontend callback URL, not the backend's server-side redirect URI.
	for _, origin := range append(cfg.CORSAllowedOrigins, cfg.FrontendURL) {
		if origin != "" {
			auth.AllowedRedirectURIs[origin+"/auth/callback"] = true
		}
	}

	// AI Provider — Gemini primary (cost-effective), Claude fallback.
	// Override with AI_PROVIDER=claude to reverse priority.
	var aiProvider ai.Provider
	{
		// Resolve Vertex AI project ID: explicit env > Firestore project (same GCP project).
		vertexProjectID := cfg.VertexAIProjectID
		if vertexProjectID == "" {
			vertexProjectID = cfg.FirestoreProjectID
		}

		var geminiProvider ai.Provider
		var claudeProvider ai.Provider

		// Prefer Developer API when API key is available;
		// fall back to Vertex AI (global endpoint) otherwise.
		// Both paths use Gemini 3 models.
		if gp, err := ai.NewGeminiProvider(cfg.GeminiAPIKey, vertexProjectID, cfg.VertexAILocation); err != nil {
			slog.Warn("failed to initialize Gemini provider", "error", err)
		} else {
			geminiProvider = gp
		}

		if cfg.AnthropicAPIKey != "" {
			claudeProvider = ai.NewClaudeProvider(cfg.AnthropicAPIKey)
			slog.Info("AI provider initialized", "provider", "claude")
		}

		switch cfg.AIProvider {
		case "claude":
			if claudeProvider != nil {
				aiProvider = ai.NewFallbackProvider(claudeProvider, geminiProvider)
				slog.Info("AI provider priority", "primary", "claude", "fallback", "gemini")
			} else if geminiProvider != nil {
				aiProvider = geminiProvider
				slog.Warn("AI_PROVIDER=claude but ANTHROPIC_API_KEY not set, using Gemini only")
			}
		default: // "gemini" or unset
			if geminiProvider != nil {
				aiProvider = ai.NewFallbackProvider(geminiProvider, claudeProvider)
				slog.Info("AI provider priority", "primary", "gemini", "fallback", "claude")
			} else if claudeProvider != nil {
				aiProvider = claudeProvider
				slog.Warn("Gemini unavailable, using Claude only")
			}
		}
	}

	// Tool registry — register global tools available in all chat modes.
	// web_search is ALWAYS registered: when configured it hits Google Custom
	// Search; when keys are missing it returns a success-shaped
	// "no_web_access" response (NOT an error) so the AI never treats it as
	// a failure, never retries, and falls back to parametric knowledge with
	// a clear caveat (#194, Run 4 R-16).
	toolRegistry := tools.NewRegistry()
	if cfg.GoogleCustomSearchAPIKey != "" && cfg.GoogleCustomSearchCX != "" {
		toolRegistry.Register(tools.NewWebSearch(cfg.GoogleCustomSearchAPIKey, cfg.GoogleCustomSearchCX))
		slog.Info("web_search tool registered (Google Custom Search backend)")
	} else {
		toolRegistry.Register(tools.NewWebSearchStub())
		slog.Warn("web_search tool registered as stub — GOOGLE_CUSTOM_SEARCH_API_KEY or GOOGLE_CUSTOM_SEARCH_CX not set; tool will return 'no_web_access' to the AI")
	}

	// Chat store
	chatStr := chatstore.New(firestoreClient)

	// Personas — composer generates expert identities via AI, falls back to templates
	var identityGen persona.IdentityGenerator
	if aiProvider != nil {
		identityGen = newAIIdentityGenerator(aiProvider)
	}
	personaComposer := persona.NewComposer(identityGen)
	personaRegistry := persona.NewRegistry(personaComposer)

	// Theme tagger — uses AI to classify trip themes
	var themeSvc *theme.Service
	if aiProvider != nil {
		tagger := persona.NewThemeTagger(newSimpleChatFn(aiProvider))
		themeSvc = theme.NewService(pool, tagger)
	}

	// Affiliate link builder — generates partner URLs for booking recommendations.
	// Empty IDs disable affiliate tracking (plain URLs still work).
	linkBuilder := affiliate.NewLinkBuilder(affiliate.LinkBuilderConfig{
		SkyscannerID:       cfg.SkyscannerAffiliateID,
		BookingComID:       cfg.BookingComAffiliateID,
		GetYourGuideID:     cfg.GetYourGuidePartnerID,
		ViatorID:           cfg.ViatorPartnerID,
		DiscoverCarsID:     cfg.DiscoverCarsAffiliateID,
		SafetyWingID:       cfg.SafetyWingReferenceID,
		ExpediaPublisherID: cfg.ExpediaPublisherID,
	})

	if aiProvider == nil {
		slog.Error("no AI provider available — neither ANTHROPIC_API_KEY nor Vertex AI credentials are configured")
		os.Exit(1)
	}

	// Services
	tripSvc := trip.NewService(pool)
	chatSvc := chat.NewService(aiProvider, chatStr, toolRegistry, personaRegistry)

	// LLM response cache — avoids redundant LLM calls for popular destination intros.
	if cfg.LLMCacheEnabled {
		responseCache := ai.NewResponseCache(ai.WithTTL(cfg.LLMCacheTTL))
		chatSvc.SetCache(responseCache)
		slog.Info("llm response cache enabled", "ttl", cfg.LLMCacheTTL)
	}

	// Daily token budget — legacy in-memory global limit across all AI calls.
	if cfg.DailyAITokenBudget > 0 {
		tokenBudget := ai.NewTokenBudget(cfg.DailyAITokenBudget)
		chatSvc.SetBudget(tokenBudget)
		slog.Info("daily AI token budget configured (legacy)", "limit", cfg.DailyAITokenBudget)
	}

	// DB-backed daily cost budget — hard limit on total AI spend per day.
	// Reads actual cost from the ai_usage table and enforces global + per-tier limits.
	var budgetChecker *usage.BudgetChecker
	if cfg.AIDailyBudgetCents > 0 {
		budgetCfg := usage.BudgetConfig{
			GlobalDailyCents: int64(cfg.AIDailyBudgetCents),
			FreePct:          cfg.AIBudgetFreePct,
			ProPct:           cfg.AIBudgetProPct,
			ExplorerPct:      cfg.AIBudgetExplorerPct,
			VoyagerPct:       cfg.AIBudgetVoyagerPct,
		}
		budgetChecker = usage.NewBudgetChecker(budgetCfg, dbgen.New(pool))
		chatSvc.SetBudgetChecker(budgetChecker)
		slog.Info("daily AI cost budget configured",
			"budget_cents", cfg.AIDailyBudgetCents,
			"free_pct", cfg.AIBudgetFreePct,
			"pro_pct", cfg.AIBudgetProPct,
			"explorer_pct", cfg.AIBudgetExplorerPct,
			"voyager_pct", cfg.AIBudgetVoyagerPct,
		)
	}

	bookingSvc := booking.NewService(pool, aiProvider)
	locationSvc := location.NewService(cfg.GooglePlacesAPIKey)
	locationCache := location.NewCache(location.DefaultCacheTTL)
	lifecycleSvc := lifecycle.NewService(pool, chatStr)

	// GDPR export storage — GCS in production, local filesystem for development.
	if cfg.GCSExportBucket != "" {
		gcsClient, gcsErr := gcstorage.NewClient(ctx)
		if gcsErr != nil {
			slog.Error("failed to create GCS client for export storage", "error", gcsErr)
		} else {
			lifecycleSvc.SetExportStore(exportstorage.NewGCSStore(gcsClient, cfg.GCSExportBucket))
			slog.Info("GDPR export storage: GCS", "bucket", cfg.GCSExportBucket)
		}
	} else {
		localStore, localErr := exportstorage.NewLocalStore(cfg.ExportLocalDir)
		if localErr != nil {
			slog.Error("failed to create local export store", "error", localErr)
		} else {
			lifecycleSvc.SetExportStore(localStore)
			slog.Info("GDPR export storage: local filesystem", "dir", cfg.ExportLocalDir)
		}
	}
	usageSvc := usage.NewService(pool, cfg.DailyMessageLimit).
		WithTierLimits(cfg.DailyMessageLimitFree, cfg.DailyMessageLimitPro)
	chatSvc.SetUsageService(usageSvc)
	if otelMetrics != nil {
		chatSvc.SetAITokenMetric(otelMetrics.AITokenUsage)
	}

	// Interceptors — handles both unary and streaming RPCs
	rateLimiter := ratelimit.NewInterceptor(10, 60)
	if otelMetrics != nil {
		rateLimiter.SetRateLimitHitsMetric(otelMetrics.RateLimitHits)
	}
	defer rateLimiter.Stop()

	queries := dbgen.New(pool)

	ageCheckFn := auth.AgeCheckFunc(func(ctx context.Context, userID uuid.UUID) (bool, error) {
		return queries.IsAgeVerified(ctx, userID)
	})

	// Consent gate: refuses non-exempt RPCs until the user has recorded
	// both `terms` and `privacy_policy` consents (see db/queries/consents.sql
	// `HasRequiredConsents`). The login response already carries a
	// `consent_pending` hint for the frontend; this interceptor enforces
	// it server-side so a client that skips the consent modal can't just
	// keep calling API methods. Finding: #369 P1 #3.
	//
	// Enforcement is gated behind CONSENT_ENFORCEMENT_ENABLED so the
	// code can ship dark, be verified against the frontend in staging,
	// then flipped on in prod. Merging the interceptor without the
	// frontend handler would brick every existing user who hadn't
	// recorded consent.
	consentCheckFn := auth.ConsentCheckFunc(func(ctx context.Context, userID uuid.UUID) (bool, error) {
		return queries.HasRequiredConsents(ctx, userID)
	})

	interceptorList := []connect.Interceptor{
		validate.NewInterceptor(),
		auth.NewAuthInterceptor(authSvc),
		auth.NewAgeInterceptor(ageCheckFn),
	}
	if cfg.ConsentEnforcementEnabled {
		slog.Info("consent enforcement interceptor enabled")
		interceptorList = append(interceptorList, auth.NewConsentInterceptor(consentCheckFn))
	} else {
		slog.Warn("consent enforcement interceptor is DISABLED — set CONSENT_ENFORCEMENT_ENABLED=true to enforce")
	}
	interceptorList = append(interceptorList, rateLimiter)
	interceptors := connect.WithInterceptors(interceptorList...)

	// Register handlers
	mux := http.NewServeMux()

	// Auth lockout — block IPs after 5 failed auth attempts within 15 minutes, for 15 minutes.
	authLimiter := ratelimit.NewAuthLimiter(5, 15*time.Minute, 15*time.Minute)
	defer authLimiter.Stop()

	paymentSvc := payment.NewService(cfg.StripeSecretKey, cfg.StripeTripProProductID, cfg.TripProPriceCents, queries, cfg.FrontendURL).
		WithAnalytics(posthogClient)
	if cfg.StagingProAll {
		if cfg.TargetEnv != "staging" {
			slog.Error("STAGING_PRO_ALL=true is only allowed in staging environment, ignoring", "env", cfg.TargetEnv)
		} else {
			paymentSvc.SetAlwaysUnlocked(true)
			slog.Info("staging mode: all trips treated as unlocked (pro)")
		}
	}

	// Stripe subscription service — no-ops gracefully when STRIPE_SECRET_KEY is empty.
	subSvc := subscription.NewService(cfg.StripeSecretKey, queries, subscription.ProductConfig{
		ExplorerMonthly: cfg.StripeExplorerMonthlyProductID,
		ExplorerAnnual:  cfg.StripeExplorerAnnualProductID,
		VoyagerMonthly:  cfg.StripeVoyagerMonthlyProductID,
		VoyagerAnnual:   cfg.StripeVoyagerAnnualProductID,
	}, cfg.FrontendURL)
	subSvc.SetPaymentService(paymentSvc)

	authHandler := handlers.NewAuthHandler(authSvc, pool, lifecycleSvc, cfg.AllowedEmailDomains, authLimiter).
		WithCapacityCap(cfg.AllowedEmails, cfg.MaxFreeUsers).
		WithFacebookCredentials(cfg.FacebookClientID, cfg.FacebookClientSecret).
		WithAnalytics(posthogClient).
		WithAlertChecker(alertChecker)
	tripHandler := handlers.NewTripHandler(tripSvc, lifecycleSvc, themeSvc, dbgen.New(pool)).
		WithAnalytics(posthogClient)
	chatHandler := handlers.NewChatHandler(chatSvc, tripSvc, themeSvc, locationCache, locationSvc, linkBuilder, usageSvc, paymentSvc, pool, cfg.AdminEmails).
		WithPlacesAPIKey(cfg.GooglePlacesAPIKey).
		WithAnalytics(posthogClient).
		WithAlertChecker(alertChecker).
		WithAIProvider(aiProvider)
	bookingHandler := handlers.NewBookingHandler(bookingSvc, queries)
	locationHandler := handlers.NewLocationHandler(locationSvc, locationCache)
	personaHandler := handlers.NewPersonaHandler(personaRegistry, pool)
	secureCookies := cfg.TargetEnv != "local"
	if cfg.TargetEnv != "local" {
		auth.SetAuthCookieDomain(".toqui.travel")
	}
	// Email sender for transactional emails (verification, invites, welcome emails)
	var emailSender *email.Sender
	if cfg.ResendAPIKey != "" {
		emailSender = email.NewSender(cfg.ResendAPIKey, cfg.EmailFrom)
	} else if cfg.TargetEnv != "local" {
		slog.Warn("RESEND_API_KEY not configured — transactional emails will be skipped")
	}
	oauthHandler := handlers.NewOAuthHandler(authSvc, pool, cfg.FrontendURL, secureCookies, cfg.AllowedEmailDomains, cfg.AllowedEmails, authLimiter, emailSender).
		WithMaxFreeUsers(cfg.MaxFreeUsers).
		WithFacebookOAuth(cfg.FacebookClientID, cfg.FacebookClientSecret, cfg.FacebookRedirectURI).
		WithAnalytics(posthogClient).
		WithAlertChecker(alertChecker)

	usageHandler := handlers.NewUsageHandler(usageSvc, authSvc, pool)

	// Shared trip handler (public + authenticated routes)
	sharedHandler := handlers.NewSharedHandler(tripSvc, authSvc, cfg.FrontendURL).
		WithBookingService(bookingSvc).
		WithAnalytics(posthogClient)

	// Liveness probe (no auth, no external checks).
	// Used by Cloud Run to verify the process is alive — never killed due to transient DB issues.
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"alive"}`))
	})

	// Health check (no auth, used by Cloud Run and load balancers).
	// Pings the database to ensure the connection pool is healthy.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			slog.Error("healthz: database ping failed", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unhealthy","reason":"database"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Readiness probe (no auth) — Kubernetes/Cloud Run uses this to gate
	// traffic during rollout. Returns 503 until the database is reachable
	// (#185).
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not_ready","reason":"database"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Public waitlist status (no auth) — looks up an email's position in the
	// waitlist queue. Documented in CLAUDE.md but was previously unregistered
	// (#186). Returns {"position":N,"total":M} for all emails. Unknown emails
	// receive {"position":0,"total":0} to prevent email enumeration attacks.
	mux.HandleFunc("/waitlist/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		email := strings.TrimSpace(r.URL.Query().Get("email"))
		if email == "" {
			http.Error(w, "email query parameter required", http.StatusBadRequest)
			return
		}
		entry, err := queries.GetWaitlistByEmail(r.Context(), email)
		if err != nil {
			// Return 200 with zeroed response to prevent email enumeration.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"position":0,"total":0}`))
			return
		}
		total, _ := queries.CountWaitlist(r.Context())
		var position int64
		if entry.AcceptedAt.Valid {
			position = 0
		} else if entry.VerifiedAt.Valid {
			ahead, _ := queries.CountWaitlistAhead(r.Context(), entry.SignedUpAt)
			position = ahead + 1
		} else {
			position = -1 // unverified
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"position":%d,"total":%d}`, position, total)
	})

	// Debug/profiling endpoints — local only, never exposed in staging/prod.
	if cfg.TargetEnv == "local" {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	// Auth HTTP routes (outside ConnectRPC)
	mux.HandleFunc("/auth/google/login", oauthHandler.HandleLogin)
	mux.HandleFunc("/auth/google/callback", oauthHandler.HandleCallback)
	mux.HandleFunc("/auth/facebook/login", oauthHandler.HandleFacebookLogin)
	mux.HandleFunc("/auth/facebook/callback", oauthHandler.HandleFacebookCallback)
	mux.HandleFunc("/auth/exchange", oauthHandler.HandleExchange)
	mux.HandleFunc("/auth/refresh", oauthHandler.HandleRefresh)
	mux.HandleFunc("/auth/logout", oauthHandler.HandleLogout)

	// Age verification route (authenticated)
	ageVerifyHandler := handlers.NewAgeVerifyHandler(authSvc, queries, lifecycleSvc)
	mux.HandleFunc("/auth/verify-age", ageVerifyHandler.HandleVerifyAge)

	// Health checks (public, no auth — for load balancer probes)
	healthHandler := handlers.NewHealthHandler(pool, startTime)
	mux.HandleFunc("/health", healthHandler.HandleHealth)
	mux.HandleFunc("/ready", healthHandler.HandleReadiness)

	// Well-known files for deep linking (public, no auth)
	mux.HandleFunc("/.well-known/apple-app-site-association", handlers.HandleAppleAppSiteAssociation)
	mux.HandleFunc("/.well-known/assetlinks.json", handlers.HandleAssetLinks)

	// Usage route (authenticated via Bearer token)
	mux.HandleFunc("/api/usage", usageHandler.HandleUsage)

	// Payment routes (authenticated)
	checkoutHandler := handlers.NewCheckoutHandler(paymentSvc, authSvc, pool).
		WithAnalytics(posthogClient)
	mux.HandleFunc("/api/checkout", checkoutHandler.HandleCreateCheckout)
	mux.HandleFunc("/api/checkout/status", checkoutHandler.HandleCheckUnlock)

	// Subscription routes (authenticated, except webhook)
	subscriptionHandler := handlers.NewSubscriptionHandler(subSvc, authSvc, pool, cfg.StripeWebhookSecret).
		WithAnalytics(posthogClient)
	mux.HandleFunc("/api/subscription/checkout", subscriptionHandler.HandleCreateCheckout)
	mux.HandleFunc("/api/subscription/cancel", subscriptionHandler.HandleCancelSubscription)
	mux.HandleFunc("/api/subscription/webhook", subscriptionHandler.HandleWebhook)
	mux.HandleFunc("/api/subscription/portal", subscriptionHandler.HandleCreatePortal)
	mux.HandleFunc("/api/subscription", subscriptionHandler.HandleGetSubscription)

	// Public destination guides (no auth required)
	guidesHandler := handlers.NewGuidesHandler(cfg.FrontendURL)
	mux.HandleFunc("/api/guides/", guidesHandler.HandleGetGuide)
	mux.HandleFunc("/api/guides", guidesHandler.HandleListGuides)

	// User feedback
	feedbackHandler := handlers.NewFeedbackHandler(authSvc, pool)
	mux.HandleFunc("/api/feedback", feedbackHandler.HandleSubmitFeedback)

	// Referral system
	referralHandler := handlers.NewReferralHandler(authSvc, pool, cfg.FrontendURL, cfg.ReferralMaxRewards).
		WithAnalytics(posthogClient)
	mux.HandleFunc("/api/referral", referralHandler.HandleGetReferralCode)
	mux.HandleFunc("/api/referral/redeem", referralHandler.HandleRedeemReferral)

	// Affiliate click tracking (public — no auth required)
	affiliateHandler := handlers.NewAffiliateHandler(posthogClient)
	mux.HandleFunc("/api/affiliate/click", affiliateHandler.HandleClick)

	// Destination autocomplete (public — no auth required)
	destinationHandler := handlers.NewDestinationSearchHandler()
	mux.HandleFunc("/api/destinations/search", destinationHandler.HandleSearch)

	// Cross-trip search endpoints (authenticated)
	searchHandler := handlers.NewSearchHandler(authSvc, pool)
	mux.HandleFunc("/api/search/itinerary", searchHandler.HandleSearchItinerary)
	mux.HandleFunc("/api/search/bookings", searchHandler.HandleSearchBookings)

	// Data export download (GDPR Article 20)
	mux.HandleFunc("/api/export/", authHandler.HandleExportDownload)

	// Privacy consent management (GDPR/PIPEDA compliance)
	consentHandler := handlers.NewConsentHandler(authSvc, pool)
	mux.HandleFunc("/auth/consent", consentHandler.HandleBatchConsent)
	mux.HandleFunc("/api/privacy/consents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			consentHandler.HandleGetConsents(w, r)
		case http.MethodPost:
			consentHandler.HandleRecordConsent(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/privacy/consents/", consentHandler.HandleWithdrawConsent)

	// User preferences (authenticated)
	preferencesHandler := handlers.NewPreferencesHandler(authSvc, pool)
	mux.HandleFunc("/api/preferences", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			preferencesHandler.HandleGetPreferences(w, r)
		case http.MethodPut:
			preferencesHandler.HandlePutPreferences(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Export handlers (authenticated)
	icalHandler := handlers.NewICalExportHandler(tripSvc, authSvc, pool)
	pdfHandler := handlers.NewPDFExportHandler(tripSvc, authSvc, pool)

	// Offline bundle handler (authenticated) — returns a complete trip
	// snapshot for offline companion mode (issue #11).
	bundleHandler := handlers.NewBundleHandler(tripSvc, bookingSvc, authSvc, chatStr, themeSvc, pool, guidesHandler)

	// Trip collaboration routes (authenticated)
	collabHandler := handlers.NewCollaborateHandler(authSvc, pool, emailSender, cfg.FrontendURL)
	mux.HandleFunc("/api/trips/accept-invite", collabHandler.HandleAcceptInvite)
	// Pattern-based routes for trip-specific collaboration endpoints.
	// Go 1.22+ ServeMux doesn't support path params, so we use prefix matching
	// and parse the trip ID in the handler.
	mux.HandleFunc("/api/trips/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/export/ical") && r.Method == http.MethodGet:
			icalHandler.HandleExportICal(w, r)
		case strings.HasSuffix(path, "/export/pdf") && r.Method == http.MethodGet:
			pdfHandler.HandleExportPDF(w, r)
		case strings.HasSuffix(path, "/bundle") && r.Method == http.MethodGet:
			bundleHandler.HandleBundle(w, r)
		case strings.HasSuffix(path, "/invite") && r.Method == http.MethodPost:
			collabHandler.HandleInvite(w, r)
		case strings.HasSuffix(path, "/collaborators") && r.Method == http.MethodGet:
			collabHandler.HandleListCollaborators(w, r)
		case strings.Contains(path, "/collaborators/") && r.Method == http.MethodDelete:
			collabHandler.HandleRemoveCollaborator(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Shared trip routes
	mux.HandleFunc("/api/trips/share", sharedHandler.HandleEnable)    // POST — enable sharing (auth)
	mux.HandleFunc("/api/trips/unshare", sharedHandler.HandleDisable) // POST — disable sharing (auth)
	mux.HandleFunc("/shared/", sharedHandler.HandlePublicView)        // GET — public view (no auth)

	// Admin UI (embedded SPA)
	mux.Handle("/admin-ui/", http.StripPrefix("/admin-ui/", http.FileServer(http.FS(adminStaticFS()))))
	mux.HandleFunc("/admin-ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin-ui/", http.StatusMovedPermanently)
	})

	// Admin endpoints (authenticated + admin email check)
	adminHandler := handlers.NewAdminHandler(authSvc, pool, cfg.AdminEmails, emailSender, cfg.FrontendURL, lifecycleSvc)
	if budgetChecker != nil {
		adminHandler.SetBudgetChecker(budgetChecker)
	}
	mux.HandleFunc("/admin/stats", adminHandler.HandleStats)
	mux.HandleFunc("/admin/users", adminHandler.HandleListUsers)
	mux.HandleFunc("/admin/waitlist", adminHandler.HandleListWaitlist)
	mux.HandleFunc("/admin/invite", adminHandler.HandleGenerateInvite)
	mux.HandleFunc("/admin/send-invite", adminHandler.HandleSendInvite)
	mux.HandleFunc("/admin/revoke-invite", adminHandler.HandleRevokeInvite)
	mux.HandleFunc("/admin/delete-waitlist", adminHandler.HandleDeleteWaitlistEntry)
	mux.HandleFunc("/admin/unlock-trip", adminHandler.HandleUnlockTrip)
	mux.HandleFunc("/admin/grant-pro", adminHandler.HandleGrantPro)
	mux.HandleFunc("/admin/delete-user", adminHandler.HandleDeleteUser)
	mux.HandleFunc("/admin/metrics", adminHandler.HandleMetrics)
	mux.HandleFunc("/admin/feedback", adminHandler.HandleListFeedback)
	mux.HandleFunc("/admin/ai-costs", adminHandler.HandleAICosts)
	mux.HandleFunc("/admin/revenue", adminHandler.HandleRevenue)
	mux.HandleFunc("/admin/set-admin", adminHandler.HandleSetAdmin)
	mux.HandleFunc("/admin/retention", adminHandler.HandleRetention)
	mux.HandleFunc("/admin/funnel", adminHandler.HandleFunnel)

	// Email ingestion webhook (outside ConnectRPC)
	emailInbound := email.NewInbound(cfg.ResendAPIKey)
	emailWebhookHandler := handlers.NewEmailWebhookHandler(bookingSvc, tripSvc, paymentSvc, pool, emailInbound, cfg.EmailWebhookSecret)
	mux.HandleFunc("/webhooks/email/inbound", emailWebhookHandler.HandleInbound)

	mux.Handle(toquiv1connect.NewAuthServiceHandler(authHandler, interceptors))
	mux.Handle(toquiv1connect.NewTripServiceHandler(tripHandler, interceptors))
	mux.Handle(toquiv1connect.NewChatServiceHandler(chatHandler, interceptors))
	mux.Handle(toquiv1connect.NewBookingServiceHandler(bookingHandler, interceptors))
	mux.Handle(toquiv1connect.NewLocationServiceHandler(locationHandler, interceptors))
	mux.Handle(toquiv1connect.NewPersonaServiceHandler(personaHandler, interceptors))

	// gRPC reflection — enables service discovery for tools like grpcurl and Bruno.
	// Disabled in production to avoid exposing the full API surface to attackers (#237).
	if cfg.TargetEnv == "local" || cfg.TargetEnv == "staging" {
		reflector := grpcreflect.NewStaticReflector(
			toquiv1connect.AuthServiceName,
			toquiv1connect.TripServiceName,
			toquiv1connect.ChatServiceName,
			toquiv1connect.BookingServiceName,
			toquiv1connect.LocationServiceName,
			toquiv1connect.PersonaServiceName,
		)
		mux.Handle(grpcreflect.NewHandlerV1(reflector))
		mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	}

	// Per-IP rate limiting — 120 requests/min sustained, burst of 20.
	// Applies to all routes (public + authenticated). The per-user ConnectRPC
	// interceptor provides tighter limits on authenticated AI endpoints.
	ipLimiter := ratelimit.NewIPRateLimiter(120, 20)
	if otelMetrics != nil {
		ipLimiter.SetRateLimitHitsMetric(otelMetrics.RateLimitHits)
	}
	defer ipLimiter.Stop()

	// Build CORS allowed origins: use CORS_ALLOWED_ORIGINS if set, otherwise
	// fall back to FRONTEND_URL only. This ensures a strict allowlist in all envs.
	corsOrigins := cfg.CORSAllowedOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{cfg.FrontendURL}
	}

	// CSRF protection — validate Origin/Referer on state-changing requests.
	// Webhooks are exempt (they use ECDSA signature verification).
	csrfProtected := csrf.Middleware(mux, corsOrigins, []string{"/webhooks/", "/api/subscription/webhook"})

	// Middleware chain: recovery → request ID → request logging → security headers → CORS → cookie auth → IP rate limit → CSRF → metrics → handler
	// IP rate limiter runs after cookie auth so it can use Bearer token (set by
	// CookieAuth for web browsers) as the rate limit key instead of spoofable X-Forwarded-For.
	// otelhttp wraps the outermost layer to trace all incoming HTTP requests.
	// telemetry.Middleware sits inside the chain to record request duration/count with accurate status codes.
	inner := recoveryMiddleware(requestid.Middleware(requestLoggingMiddleware(securityHeadersMiddleware(corsMiddleware(middleware.CookieAuth(ipLimiter.Middleware(telemetry.Middleware(otelMetrics, csrfProtected))), corsOrigins)))))

	// Only wrap with otelhttp when an OTLP exporter endpoint is configured.
	// otelhttp's response-writer wrapper does not propagate http.Flusher, which
	// breaks ConnectRPC server-streaming RPCs (e.g. ChatService/SendMessage).
	var handler http.Handler
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		handler = otelhttp.NewHandler(inner, "toqui-backend")
	} else {
		handler = inner
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h2c.NewHandler(handler, &http2.Server{}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      300 * time.Second, // long for SSE streaming responses
		IdleTimeout:       120 * time.Second,
	}

	// Background lifecycle jobs — token cleanup, trip archival, deletion retries.
	// Cancelling jobsCtx signals the jobs goroutine to stop gracefully.
	jobsCtx, jobsCancel := context.WithCancel(context.Background())
	defer jobsCancel()
	lifecycleJobs := lifecycle.NewJobs(lifecycleSvc, pool)
	go lifecycleJobs.Start(jobsCtx)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		// Stop background jobs first.
		jobsCancel()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		slog.Info("shutting down server")
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}

		// Drain PostHog event queue after server stops accepting new requests.
		slog.Info("draining PostHog event queue")
		posthogClient.Close()
	}()

	slog.Info("server starting", "port", cfg.Port, "env", cfg.TargetEnv)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func newFirestoreClient(ctx context.Context, cfg *config.Config) (*firestore.Client, error) {
	if cfg.FirestoreEmulatorHost != "" {
		os.Setenv("FIRESTORE_EMULATOR_HOST", cfg.FirestoreEmulatorHost)
	}
	if cfg.FirestoreDatabaseID != "" {
		client, err := firestore.NewClientWithDatabase(ctx, cfg.FirestoreProjectID, cfg.FirestoreDatabaseID)
		if err != nil {
			return nil, fmt.Errorf("create firestore client (database %s): %w", cfg.FirestoreDatabaseID, err)
		}
		return client, nil
	}
	client, err := firestore.NewClient(ctx, cfg.FirestoreProjectID)
	if err != nil {
		return nil, fmt.Errorf("create firestore client: %w", err)
	}
	return client, nil
}

// newAIIdentityGenerator creates an IdentityGenerator that uses the AI provider
// to generate persona names, descriptions, and greetings for new location+theme combos.
func newAIIdentityGenerator(provider ai.Provider) persona.IdentityGenerator {
	return func(ctx context.Context, req *persona.IdentityRequest) (*persona.IdentityResult, error) {
		prompt := persona.IdentityGeneratorPrompt(req)

		aiReq := &ai.ChatRequest{
			SystemPrompt: "You are a creative writer who generates character identities for AI travel guides. Respond with JSON only.",
			Messages: []ai.Message{
				{Role: "user", Content: prompt},
			},
			MaxTokens:   256,
			Temperature: 0.8,
			ModelTier:   ai.ModelTierFast,
		}

		eventCh, err := provider.ChatStream(ctx, aiReq)
		if err != nil {
			return nil, fmt.Errorf("AI identity generation: %w", err)
		}

		var response strings.Builder
		for event := range eventCh {
			switch event.Type {
			case ai.EventTextDelta:
				response.WriteString(event.Text)
			case ai.EventError:
				return nil, fmt.Errorf("AI identity generation stream error: %w", event.Error)
			}
		}

		return persona.ParseIdentityResult(response.String())
	}
}

// newSimpleChatFn wraps a streaming AI provider into a simple request/response function
// for use by the theme tagger and other non-streaming AI consumers.
func newSimpleChatFn(provider ai.Provider) func(ctx context.Context, system, prompt string) (string, error) {
	return func(ctx context.Context, system, prompt string) (string, error) {
		req := &ai.ChatRequest{
			SystemPrompt: system,
			Messages:     []ai.Message{{Role: "user", Content: prompt}},
			MaxTokens:    256,
			Temperature:  0.3,
			ModelTier:    ai.ModelTierFast,
		}

		eventCh, err := provider.ChatStream(ctx, req)
		if err != nil {
			return "", err
		}

		var response strings.Builder
		for event := range eventCh {
			switch event.Type {
			case ai.EventTextDelta:
				response.WriteString(event.Text)
			case ai.EventError:
				return "", event.Error
			}
		}
		return response.String(), nil
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
// It also implements http.Flusher so server-streaming RPCs (e.g. chat)
// can flush chunks to the client.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// requestLoggingMiddleware logs each HTTP request with method, path, status, and
// duration. In non-local environments these become structured JSON entries in
// Cloud Logging, queryable for dashboards and alerting.
func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip noisy health/readiness probes.
		if r.URL.Path == "/healthz" || r.URL.Path == "/livez" || r.URL.Path == "/readyz" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := time.Since(start)

		level := slog.LevelInfo
		if sw.status >= 500 {
			level = slog.LevelError
		} else if sw.status >= 400 {
			level = slog.LevelWarn
		}

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", duration.Milliseconds(),
			"user_agent", r.Header.Get("User-Agent"),
		}
		if reqID := requestid.FromContext(r.Context()); reqID != "" {
			attrs = append(attrs, "request_id", reqID)
		}

		slog.Log(r.Context(), level, "http_request", attrs...)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"error", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// Admin SPA needs inline scripts/styles and fetch access to same origin.
		if strings.HasPrefix(r.URL.Path, "/admin-ui") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		} else {
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}
		// HSTS — only set on HTTPS (Cloud Run terminates TLS)
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	// Build a set for fast lookup (lowercased for case-insensitive matching).
	originSet := make(map[string]string, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[strings.ToLower(o)] = o
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Always set Vary: Origin so caches don't serve a CORS response
		// to a non-CORS request or vice versa.
		w.Header().Set("Vary", "Origin")

		// Only set CORS headers if the request Origin matches our allowlist.
		// When AllowCredentials is true, we must echo the specific matched origin
		// (wildcard "*" is not allowed with credentials).
		if origin != "" {
			if matched, ok := originSet[strings.ToLower(origin)]; ok {
				w.Header().Set("Access-Control-Allow-Origin", matched)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Connect-Protocol-Version")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "7200")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
