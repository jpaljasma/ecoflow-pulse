import { mockDevices } from '@/features/devices/mockData';

type MockResponse = {
  status: number;
  body: unknown;
};

export function maybeHandleMockRequest(path: string): MockResponse | null {
  if (path === '/api/devices') {
    return {
      status: 200,
      body: { devices: mockDevices }
    };
  }

  const match = path.match(/^\/api\/devices\/([^/]+)$/);
  if (match) {
    const id = decodeURIComponent(match[1] as string);
    const device = mockDevices.find((d) => d.id === id);
    if (!device) {
      return { status: 404, body: { message: 'Device not found' } };
    }
    return { status: 200, body: device };
  }

  return { status: 404, body: { message: `Mock route not found: ${path}` } };
}
