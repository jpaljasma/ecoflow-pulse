export type InventoryMetricLayoutItem = {
  key: 'ac' | 'dc' | 'pv' | 'load' | 'net';
  span: number;
};

export function buildInventoryMetricLayout(): {
  columns: number;
  items: InventoryMetricLayoutItem[];
} {
  return {
    columns: 6,
    items: [
      { key: 'ac', span: 2 },
      { key: 'dc', span: 2 },
      { key: 'pv', span: 2 },
      { key: 'load', span: 3 },
      { key: 'net', span: 3 }
    ]
  };
}

export function buildStatusDotHoverLabel(lastSeenLabel: string, statusLabel: string | null): string {
  return statusLabel ? `${statusLabel} · ${lastSeenLabel}` : lastSeenLabel;
}
