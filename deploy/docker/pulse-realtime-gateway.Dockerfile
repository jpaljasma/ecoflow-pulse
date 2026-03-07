FROM node:20-bookworm-slim AS build

WORKDIR /app

COPY package.json package-lock.json ./
COPY tsconfig.json ./
COPY apps/pulse-realtime-gateway/package.json ./apps/pulse-realtime-gateway/package.json
COPY packages/node-jwks-auth/package.json ./packages/node-jwks-auth/package.json
COPY packages/tsconfig/package.json ./packages/tsconfig/package.json

RUN node -e "const fs=require('fs'); const pkg=JSON.parse(fs.readFileSync('package.json','utf8')); pkg.workspaces=['apps/pulse-realtime-gateway','packages/node-jwks-auth','packages/tsconfig']; fs.writeFileSync('package.json', JSON.stringify(pkg,null,2));"

RUN npm ci
COPY apps/pulse-realtime-gateway ./apps/pulse-realtime-gateway
COPY packages/node-jwks-auth ./packages/node-jwks-auth
COPY packages/tsconfig ./packages/tsconfig
COPY proto ./proto
RUN npm run build --workspace @ecoflow-pulse/node-jwks-auth
RUN npm run build --workspace @ecoflow-pulse/pulse-realtime-gateway
RUN npm prune --omit=dev

FROM node:20-bookworm-slim

WORKDIR /app

COPY package.json package-lock.json ./
COPY apps/pulse-realtime-gateway/package.json ./apps/pulse-realtime-gateway/package.json
COPY packages/node-jwks-auth/package.json ./packages/node-jwks-auth/package.json

RUN node -e "const fs=require('fs'); const pkg=JSON.parse(fs.readFileSync('package.json','utf8')); pkg.workspaces=['apps/pulse-realtime-gateway','packages/node-jwks-auth']; fs.writeFileSync('package.json', JSON.stringify(pkg,null,2));"
RUN npm ci --omit=dev --workspace @ecoflow-pulse/pulse-realtime-gateway --workspace @ecoflow-pulse/node-jwks-auth

COPY --from=build /app/apps/pulse-realtime-gateway/dist ./apps/pulse-realtime-gateway/dist
COPY --from=build /app/packages/node-jwks-auth/dist ./packages/node-jwks-auth/dist
COPY --from=build /app/proto ./proto

ENV NODE_ENV=production
ENV PULSE_REALTIME_GATEWAY_HOST=0.0.0.0
ENV PULSE_REALTIME_GATEWAY_PORT=8082

EXPOSE 8082

CMD ["node", "apps/pulse-realtime-gateway/dist/server.js"]
