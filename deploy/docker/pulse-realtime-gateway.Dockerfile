FROM node:20-bookworm-slim

WORKDIR /app

COPY package.json package-lock.json ./
COPY apps ./apps
COPY packages ./packages
COPY proto ./proto

RUN npm ci

ENV NODE_ENV=production
ENV PULSE_REALTIME_GATEWAY_HOST=0.0.0.0
ENV PULSE_REALTIME_GATEWAY_PORT=8082

EXPOSE 8082

CMD ["node", "--import", "tsx", "apps/pulse-realtime-gateway/src/server.ts"]
