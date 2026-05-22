import { z } from 'zod';
import { requestJson } from '@/shared/api/restClient';
import type { AdminLogFilterOption } from '@/features/adminLogs/model';

const AdminLogFilterOptionSchema = z.object({
  kind: z.enum(['device', 'serial', 'user']),
  id: z.string(),
  label: z.string(),
  secondaryLabel: z.string(),
  deviceIds: z.array(z.string())
});

const AdminLogFilterOptionsResponseSchema = z.object({
  options: z.array(AdminLogFilterOptionSchema)
});

export type AdminLogFilterKind = AdminLogFilterOption['kind'];

export async function fetchAdminLogFilterOptions(input: {
  token?: string;
  kind: AdminLogFilterKind;
  query: string;
  limit?: number;
}): Promise<AdminLogFilterOption[]> {
  const data = await requestJson<unknown>('/api/v1/admin/log-filter-options', {
    method: 'POST',
    token: input.token,
    body: {
      kind: input.kind,
      query: input.query,
      limit: input.limit ?? 8
    }
  });
  return AdminLogFilterOptionsResponseSchema.parse(data).options;
}
