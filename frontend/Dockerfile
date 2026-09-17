FROM node:22-alpine AS build

WORKDIR /app

ENV PNPM_CONFIG_MINIMUM_RELEASE_AGE=0

RUN corepack enable

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --config.minimum-release-age=0

COPY . .
RUN pnpm build

FROM docker.io/library/nginx:alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=5s --timeout=3s --retries=10 \
    CMD wget --quiet --tries=1 --spider http://127.0.0.1/health || exit 1
