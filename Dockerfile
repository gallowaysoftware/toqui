# ───────── build stage: produce the static web bundle ─────────────────────────
FROM node:24-alpine AS build
WORKDIR /app

# Enable pnpm via corepack and prefetch deps for caching.
RUN corepack enable && corepack prepare pnpm@10 --activate

COPY package.json pnpm-lock.yaml ./
COPY patches ./patches
RUN pnpm install --frozen-lockfile

COPY . .

# Expo's web export is fully static — no runtime Node needed at serve time.
RUN npx expo export --platform web --output-dir dist/web --clear

# ───────── serve stage: nginx with the static bundle ──────────────────────────
FROM nginx:alpine

COPY --from=build /app/dist/web /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY security-headers.conf /etc/nginx/security-headers.conf
COPY docker-entrypoint.sh /docker-entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
