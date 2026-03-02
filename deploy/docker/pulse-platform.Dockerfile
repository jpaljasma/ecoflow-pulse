FROM node:20-bookworm-slim

WORKDIR /app

COPY package.json package-lock.json ./
COPY apps ./apps
COPY packages ./packages
COPY proto ./proto

RUN npm ci
RUN npm run -w apps/universal export:web -- --output-dir dist

ENV NODE_ENV=production
ENV PULSE_PLATFORM_HOST=0.0.0.0
ENV PULSE_PLATFORM_PORT=8080
ENV PULSE_PLATFORM_PUBLIC_DIR=/app/apps/universal/dist

EXPOSE 8080

CMD ["node", "--import", "tsx", "apps/pulse-platform/src/server.ts"]
