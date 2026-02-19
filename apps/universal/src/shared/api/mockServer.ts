import { getMockDevices } from '@/shared/api/mockLogDevices';

type MockResponse = {
  status: number;
  body: unknown;
};

export async function maybeHandleMockRequest(path: string): Promise<MockResponse | null> {
  const devices = await getMockDevices();

  if (path === '/api/devices') {
    return {
      status: 200,
      body: { devices }
    };
  }

  const match = path.match(/^\/api\/devices\/([^/]+)$/);
  if (match) {
    const id = decodeURIComponent(match[1] as string);
    const device = devices.find((d) => d.id === id);
    if (!device) {
      return { status: 404, body: { message: 'Device not found' } };
    }
    return { status: 200, body: device };
  }

  return { status: 404, body: { message: `Mock route not found: ${path}` } };
}
