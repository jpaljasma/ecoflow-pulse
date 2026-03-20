// Expo web currently emits a non-module bundle, so keep Zustand imports behind
// CommonJS requires to avoid leaking import.meta into the browser runtime.
// eslint-disable-next-line @typescript-eslint/no-require-imports
const zustand = require('zustand') as typeof import('zustand');
// eslint-disable-next-line @typescript-eslint/no-require-imports
const middleware = require('zustand/middleware') as typeof import('zustand/middleware');

export const { create } = zustand;
export const { createJSONStorage, persist } = middleware;
