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
	"connectrpc.com/connect"
	"github.com/google/uuid"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/chat"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/config"
	"github.com/gallowaysoftware/toqui-backend/internal/csrf"
	"github.com/gallowaysoftware/toqui-backend/internal/db"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/handlers"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
	"github.com/gallowaysoftware/toqui-backend/internal/location"
	"github.com/gallowaysoftware/toqui-backend/internal/middleware"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
	"github.com/gallowaysoftware/toqui-backend/internal/requestid"
	"github.com/gallowaysoftware/toqui-backend/internal/theme"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
	"github.com/gallowaysoftware/toqui-backend/internal/usage"
	"github.com/gallowaysoftware/toqui-backend/internal/validate"

	toquiv1connect "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1/toquiv1connect"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	// Use JSON structured logging in non-local environments for Cloud Logging.
	// Cloud Logging automatically parses JSON from stdout and indexes severity,
	// message, and structured fields for querying and alerting.
	if cfg.TargetEnv != "local" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})))
	}

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

	// AI Provider — Claude primary, Gemini (Vertex AI) fallback
	var aiProvider ai.Provider
	if cfg.AnthropicAPIKey != "" {
		aiProvider = ai.NewClaudeProvider(cfg.AnthropicAPIKey)
		slog.Info("AI provider configured", "provider", "claude")
	} else {
		// Vertex AI fallback — uses Application Default Credentials, no API key needed.
		projectID := cfg.VertexAIProjectID
		if projectID == "" {
			projectID = cfg.FirestoreProjectID // reasonable default — same GCP project
		}
		var err error
		aiProvider, err = ai.NewGeminiProvider(projectID, cfg.VertexAILocation)
		if err != nil {
			slog.Warn("failed to initialize Gemini provider — chat will not work", "error", err)
		} else {
			slog.Info("AI provider configured", "provider", "gemini", "project", projectID, "location", cfg.VertexAILocation)
		}
	}

	// Tool registry
	toolRegistry := tools.NewRegistry()
	// Tools will be registered when API keys are configured

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
		SkyscannerID:   cfg.SkyscannerAffiliateID,
		BookingComID:   cfg.BookingComAffiliateID,
		GetYourGuideID: cfg.GetYourGuidePartnerID,
		ViatorID:       cfg.ViatorPartnerID,
		DiscoverCarsID: cfg.DiscoverCarsAffiliateID,
		SafetyWingID:   cfg.SafetyWingReferenceID,
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

	// Daily token budget — global limit across all AI calls.
	if cfg.DailyAITokenBudget > 0 {
		tokenBudget := ai.NewTokenBudget(cfg.DailyAITokenBudget)
		chatSvc.SetBudget(tokenBudget)
		slog.Info("daily AI token budget configured", "limit", cfg.DailyAITokenBudget)
	}

	bookingSvc := booking.NewService(pool, aiProvider)
	locationSvc := location.NewService()
	locationCache := location.NewCache(location.DefaultCacheTTL)
	lifecycleSvc := lifecycle.NewService(pool, chatStr)
	usageSvc := usage.NewService(pool, cfg.DailyMessageLimit)

	// Interceptors — handles both unary and streaming RPCs
	rateLimiter := ratelimit.NewInterceptor(10, 60)
	defer rateLimiter.Stop()

	queries := dbgen.New(pool)

	ageCheckFn := auth.AgeCheckFunc(func(ctx context.Context, userID uuid.UUID) (bool, error) {
		return queries.IsAgeVerified(ctx, userID)
	})

	interceptors := connect.WithInterceptors(
		validate.NewInterceptor(),
		auth.NewAuthInterceptor(authSvc),
		auth.NewAgeInterceptor(ageCheckFn),
		rateLimiter,
	)

	// Register handlers
	mux := http.NewServeMux()

	// Auth lockout — block IPs after 5 failed auth attempts within 15 minutes, for 15 minutes.
	authLimiter := ratelimit.NewAuthLimiter(5, 15*time.Minute, 15*time.Minute)
	defer authLimiter.Stop()

	paymentSvc := payment.NewService(cfg.HelcimAPIToken, cfg.TripProPriceCents, queries)

	authHandler := handlers.NewAuthHandler(authSvc, pool, lifecycleSvc, cfg.AllowedEmailDomains, cfg.AllowedEmails, cfg.MaxFreeUsers, authLimiter)
	tripHandler := handlers.NewTripHandler(tripSvc, lifecycleSvc, themeSvc)
	chatHandler := handlers.NewChatHandler(chatSvc, tripSvc, themeSvc, locationCache, locationSvc, linkBuilder, usageSvc, paymentSvc, pool)
	bookingHandler := handlers.NewBookingHandler(bookingSvc)
	locationHandler := handlers.NewLocationHandler(locationSvc, locationCache)
	personaHandler := handlers.NewPersonaHandler(personaRegistry, pool)
	secureCookies := cfg.TargetEnv != "local"
	oauthHandler := handlers.NewOAuthHandler(authSvc, pool, cfg.FrontendURL, secureCookies, cfg.MaxFreeUsers, cfg.AllowedEmailDomains, cfg.AllowedEmails, authLimiter)
	waitlistHandler := handlers.NewWaitlistHandler(pool)
	usageHandler := handlers.NewUsageHandler(usageSvc, authSvc)

	// Shared trip handler (public + authenticated routes)
	sharedHandler := handlers.NewSharedHandler(tripSvc, authSvc, cfg.FrontendURL)

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
	mux.HandleFunc("/auth/exchange", oauthHandler.HandleExchange)
	mux.HandleFunc("/auth/refresh", oauthHandler.HandleRefresh)
	mux.HandleFunc("/auth/logout", oauthHandler.HandleLogout)

	// Age verification route (authenticated)
	ageVerifyHandler := handlers.NewAgeVerifyHandler(authSvc, queries)
	mux.HandleFunc("/auth/verify-age", ageVerifyHandler.HandleVerifyAge)

	// Waitlist routes (public, no auth)
	mux.HandleFunc("/waitlist", waitlistHandler.HandleJoin)
	mux.HandleFunc("/waitlist/status", waitlistHandler.HandleStatus)

	// Usage route (authenticated via Bearer token)
	mux.HandleFunc("/api/usage", usageHandler.HandleUsage)

	// Payment routes (authenticated)
	checkoutHandler := handlers.NewCheckoutHandler(paymentSvc, authSvc, pool)
	mux.HandleFunc("/api/checkout", checkoutHandler.HandleCreateCheckout)
	mux.HandleFunc("/api/checkout/validate", checkoutHandler.HandleValidatePayment)
	mux.HandleFunc("/api/checkout/status", checkoutHandler.HandleCheckUnlock)

	// Destination guide routes (public, no auth)
	guideHandler := handlers.NewGuideHandler(newSimpleChatFn(aiProvider))
	mux.HandleFunc("/api/guides/", guideHandler.HandleGuide)
	mux.HandleFunc("/api/guides", guideHandler.HandleList)

	// Shared trip routes
	mux.HandleFunc("/api/trips/share", sharedHandler.HandleEnable)    // POST — enable sharing (auth)
	mux.HandleFunc("/api/trips/unshare", sharedHandler.HandleDisable) // POST — disable sharing (auth)
	mux.HandleFunc("/shared/", sharedHandler.HandlePublicView)        // GET — public view (no auth)

	// Admin endpoints (authenticated + admin email check)
	adminHandler := handlers.NewAdminHandler(authSvc, pool, cfg.AdminEmails)
	mux.HandleFunc("/admin/stats", adminHandler.HandleStats)
	mux.HandleFunc("/admin/users", adminHandler.HandleListUsers)
	mux.HandleFunc("/admin/waitlist", adminHandler.HandleListWaitlist)
	mux.HandleFunc("/admin/invite", adminHandler.HandleGenerateInvite)
	mux.HandleFunc("/admin/unlock-trip", adminHandler.HandleUnlockTrip)

	// Email ingestion webhook (outside ConnectRPC)
	emailWebhookHandler := handlers.NewEmailWebhookHandler(bookingSvc, tripSvc, paymentSvc, pool, cfg.SendGridWebhookKey)
	mux.HandleFunc("/webhooks/email/inbound", emailWebhookHandler.HandleInbound)

	mux.Handle(toquiv1connect.NewAuthServiceHandler(authHandler, interceptors))
	mux.Handle(toquiv1connect.NewTripServiceHandler(tripHandler, interceptors))
	mux.Handle(toquiv1connect.NewChatServiceHandler(chatHandler, interceptors))
	mux.Handle(toquiv1connect.NewBookingServiceHandler(bookingHandler, interceptors))
	mux.Handle(toquiv1connect.NewLocationServiceHandler(locationHandler, interceptors))
	mux.Handle(toquiv1connect.NewPersonaServiceHandler(personaHandler, interceptors))

	// Per-IP rate limiting — 120 requests/min sustained, burst of 20.
	// Applies to all routes (public + authenticated). The per-user ConnectRPC
	// interceptor provides tighter limits on authenticated AI endpoints.
	ipLimiter := ratelimit.NewIPRateLimiter(120, 20)
	defer ipLimiter.Stop()

	// Build CORS allowed origins: use CORS_ALLOWED_ORIGINS if set, otherwise
	// fall back to FRONTEND_URL only. This ensures a strict allowlist in all envs.
	corsOrigins := cfg.CORSAllowedOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{cfg.FrontendURL}
	}

	// CSRF protection — validate Origin/Referer on state-changing requests.
	// Webhooks are exempt (they use ECDSA signature verification).
	csrfProtected := csrf.Middleware(mux, corsOrigins, []string{"/webhooks/"})

	// Middleware chain: recovery → request ID → request logging → security headers → CORS → cookie auth → IP rate limit → CSRF → handler
	// IP rate limiter runs after cookie auth so it can use Bearer token (set by
	// CookieAuth for web browsers) as the rate limit key instead of spoofable X-Forwarded-For.
	handler := recoveryMiddleware(requestid.Middleware(requestLoggingMiddleware(securityHeadersMiddleware(corsMiddleware(middleware.CookieAuth(ipLimiter.Middleware(csrfProtected)), corsOrigins)))))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h2c.NewHandler(handler, &http2.Server{}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      300 * time.Second, // long for SSE streaming responses
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		slog.Info("shutting down server")
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
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
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// requestLoggingMiddleware logs each HTTP request with method, path, status, and
// duration. In non-local environments these become structured JSON entries in
// Cloud Logging, queryable for dashboards and alerting.
func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip noisy health probes.
		if r.URL.Path == "/healthz" || r.URL.Path == "/livez" {
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

		slog.Log(r.Context(), level, "http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", duration.Milliseconds(),
			"user_agent", r.Header.Get("User-Agent"),
		)
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
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
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
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
