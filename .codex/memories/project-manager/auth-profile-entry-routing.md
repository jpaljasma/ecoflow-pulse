# Project Manager Memory — Auth Profile Entry Routing

## Current focus

- Keep ADR-0020 implementation sequenced as one coordinated M1 follow-up instead of letting UI, auth, and authz drift independently.

## Decisions locked

- Google visible, Facebook hidden by env.
- `/profile` is standalone.
- Pulse profile edits override future social-profile claim refreshes.
- Weather location consent/timezone preferences ship in this milestone.

## Next step

- Keep schema/API work ahead of protected-route UI so the login flow has a real bootstrap target.
