export type PulseLogAccessInput = {
  roles: readonly string[] | undefined;
  deviceCount: number | undefined;
};

export function isPulseGlobalAdmin(roles: readonly string[] | undefined): boolean {
  return roles?.some((role) => role.trim().toLowerCase() === 'admin') ?? false;
}

export function canAccessPulseLogs(input: PulseLogAccessInput): boolean {
  return isPulseGlobalAdmin(input.roles) || (input.deviceCount ?? 0) > 0;
}
