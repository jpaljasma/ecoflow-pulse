export type StormGuardState = {
  active: boolean;
  endsAtUnixMs?: number;
};

export function deriveStormGuardState(input: {
  open?: boolean;
  endTimeSeconds?: number;
  nowUnixMs?: number;
}): StormGuardState {
  const nowUnixMs = input.nowUnixMs ?? Date.now();
  const endsAtUnixMs =
    input.endTimeSeconds !== undefined && input.endTimeSeconds > 0
      ? Math.trunc(input.endTimeSeconds * 1000)
      : undefined;
  const hasFutureWindow = endsAtUnixMs !== undefined && endsAtUnixMs > nowUnixMs;

  return {
    active: input.open === true || hasFutureWindow,
    endsAtUnixMs
  };
}
