export const DEVICE_INVENTORY_GRID_GAP = 16;

export function resolveDeviceInventoryGrid(contentWidth: number, pagePadding: number) {
  const columns = contentWidth >= 1120 ? 3 : contentWidth >= 768 ? 2 : 1;
  const availableWidth = Math.max(0, contentWidth - pagePadding * 2 - DEVICE_INVENTORY_GRID_GAP * (columns - 1));
  return {
    columns,
    cardWidth: Math.floor(availableWidth / columns),
    compactCard: columns > 1
  };
}
