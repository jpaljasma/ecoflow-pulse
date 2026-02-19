export type DeviceSummary = {
  id: string;
  name: string;
  model: string;
  online: boolean;
};

export type DeviceListResponse = {
  devices: DeviceSummary[];
};

export type DeviceDetailResponse = DeviceSummary & {
  capabilities?: Record<string, unknown>;
};
