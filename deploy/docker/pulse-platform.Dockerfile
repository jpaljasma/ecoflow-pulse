FROM node:20-bookworm-slim AS build

WORKDIR /app

COPY package.json package-lock.json ./
COPY tsconfig.json ./
COPY apps/pulse-platform/package.json ./apps/pulse-platform/package.json
COPY apps/universal/package.json ./apps/universal/package.json
COPY packages/api-types/package.json ./packages/api-types/package.json
COPY packages/node-jwks-auth/package.json ./packages/node-jwks-auth/package.json
COPY packages/tsconfig/package.json ./packages/tsconfig/package.json

RUN node -e "const fs=require('fs'); const pkg=JSON.parse(fs.readFileSync('package.json','utf8')); pkg.workspaces=['apps/pulse-platform','apps/universal','packages/api-types','packages/node-jwks-auth','packages/tsconfig']; fs.writeFileSync('package.json', JSON.stringify(pkg,null,2));"

RUN npm ci
COPY apps/pulse-platform ./apps/pulse-platform
COPY apps/universal ./apps/universal
COPY packages/api-types ./packages/api-types
COPY packages/node-jwks-auth ./packages/node-jwks-auth
COPY packages/tsconfig ./packages/tsconfig
COPY proto ./proto
RUN npm run build --workspace @ecoflow-pulse/node-jwks-auth
RUN npm run build --workspace @ecoflow-pulse/pulse-platform
RUN npm run -w apps/universal export:web -- --output-dir dist
RUN npm prune --omit=dev

FROM node:20-bookworm-slim

WORKDIR /app

COPY package.json package-lock.json ./
COPY apps/pulse-platform/package.json ./apps/pulse-platform/package.json
COPY packages/node-jwks-auth/package.json ./packages/node-jwks-auth/package.json

RUN node -e "const fs=require('fs'); const pkg=JSON.parse(fs.readFileSync('package.json','utf8')); pkg.workspaces=['apps/pulse-platform','packages/node-jwks-auth']; fs.writeFileSync('package.json', JSON.stringify(pkg,null,2));"
RUN npm ci --omit=dev --workspace @ecoflow-pulse/pulse-platform --workspace @ecoflow-pulse/node-jwks-auth

COPY --from=build /app/apps/pulse-platform/dist ./apps/pulse-platform/dist
COPY --from=build /app/apps/universal/dist ./apps/universal/dist
COPY --from=build /app/packages/node-jwks-auth/dist ./packages/node-jwks-auth/dist
COPY --from=build /app/proto ./proto

ENV NODE_ENV=production
ENV PULSE_PLATFORM_HOST=0.0.0.0
ENV PULSE_PLATFORM_PORT=8080
ENV PULSE_PLATFORM_PUBLIC_DIR=/app/apps/universal/dist

EXPOSE 8080

CMD ["node", "apps/pulse-platform/dist/server.js"]
