# Frontend Universal Memory — Auth Profile Entry Routing

## Current focus

- Replace the current root redirect with a proper public entry flow and introduce reusable protected-route handling before protected screens render.

## Routes in scope

- `/`
- `/login`
- `/devices`
- `/device/[deviceId]`
- `/profile`
- `/settings`

## Next step

- Build a small auth gate + `returnTo` sanitizer first so welcome/login/profile/logout work stays consistent across web and native.

