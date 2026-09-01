# syntax=docker/dockerfile:1

FROM node:24-alpine3.22 AS deps
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM node:24-alpine3.22 AS builder
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NUXT_PUBLIC_SITE_URL=https://canteiro.brenon.cloud
ARG NUXT_PUBLIC_SITE_HOST=canteiro.brenon.cloud
ENV NUXT_PUBLIC_SITE_URL=${NUXT_PUBLIC_SITE_URL}
ENV NUXT_PUBLIC_SITE_HOST=${NUXT_PUBLIC_SITE_HOST}
ENV NUXT_PUBLIC_API_BASE=""
RUN pnpm build

FROM node:24-alpine3.22 AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000
ENV NITRO_HOST=0.0.0.0
ENV NITRO_PORT=3000
RUN addgroup -g 1001 -S nodejs && adduser -S nuxt -u 1001 -G nodejs
COPY --from=builder --chown=nuxt:nodejs /app/.output ./.output
USER 1001:1001
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -q -O- http://127.0.0.1:3000/ || exit 1
ENTRYPOINT ["node", ".output/server/index.mjs"]
