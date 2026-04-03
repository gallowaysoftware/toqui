#!/bin/sh
set -e

# Generate runtime config from environment variables.
# This runs at container startup, so secrets from Cloud Run env vars
# (sourced from Secret Manager) are injected here — never baked into
# the Docker image or static JS bundles.
cat > /usr/share/nginx/html/config.json <<EOF
{
  "apiUrl": "${EXPO_PUBLIC_API_URL:-http://localhost:8090}",
  "googleClientId": "${GOOGLE_CLIENT_ID:-}",
  "posthogKey": "${POSTHOG_KEY:-}",
  "sentryDsn": "${SENTRY_DSN:-}"
}
EOF

# Start nginx
exec nginx -g 'daemon off;'
