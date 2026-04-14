export function resolveNetPowerW({
  acInW,
  pvW,
  loadW,
  dcW,
  fallbackNetW
}: {
  acInW?: number;
  pvW?: number;
  loadW?: number;
  dcW?: number;
  fallbackNetW?: number;
}): number | undefined {
  if (typeof loadW !== 'number') {
    return fallbackNetW;
  }

  return (
    (typeof acInW === 'number' ? acInW : 0) +
    (typeof pvW === 'number' ? pvW : 0) -
    loadW -
    (typeof dcW === 'number' ? dcW : 0)
  );
}
