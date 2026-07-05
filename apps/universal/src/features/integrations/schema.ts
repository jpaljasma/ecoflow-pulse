import { z } from 'zod';

export const IntegrationSchema = z.object({
  id: z.string().uuid(),
  provider: z.string(),
  accessKeyMask: z.string(),
  config: z.record(z.string(), z.unknown()).default({}),
  isActive: z.boolean(),
  createdAtUnixMs: z.string(),
  updatedAtUnixMs: z.string()
});

export const IntegrationsResponseSchema = z.object({
  integrations: z.array(IntegrationSchema)
});

export const IntegrationResponseSchema = z.object({
  integration: IntegrationSchema
});

export const CreateIntegrationPayloadSchema = z.object({
  provider: z.string().trim().min(1).max(64),
  accessKey: z.string().trim().min(1).max(512),
  accessSecret: z.string().trim().min(1).max(512),
  config: z.record(z.string(), z.unknown()).default({}),
  isActive: z.boolean().default(true)
});

export const UpdateIntegrationPayloadSchema = z.object({
  accessKey: z.string().trim().min(1).max(512),
  accessSecret: z.string().trim().min(1).max(512),
  config: z.record(z.string(), z.unknown()).default({}),
  isActive: z.boolean().default(true)
});

export const SetIntegrationActivePayloadSchema = z.object({
  isActive: z.boolean()
});

export const EcoFlowBLEAuthStatusSchema = z.object({
  connected: z.boolean(),
  status: z.string(),
  accountMask: z.string(),
  updatedAtUnixMs: z.string()
});

export const EcoFlowBLEAuthStatusResponseSchema = z.object({
  status: EcoFlowBLEAuthStatusSchema
});

export const ConnectEcoFlowBLEAuthPayloadSchema = z.object({
  email: z.string().trim().email().max(320),
  password: z.string().min(1).max(512).refine((value) => value.trim().length > 0, {
    message: 'password required'
  })
});

export const SetEcoFlowBLEAuthUserIDPayloadSchema = z.object({
  userId: z.string().trim().min(1).max(256),
  accountLabel: z.string().trim().max(128).optional()
});

export type Integration = z.infer<typeof IntegrationSchema>;
export type IntegrationsResponse = z.infer<typeof IntegrationsResponseSchema>;
export type CreateIntegrationPayload = z.infer<typeof CreateIntegrationPayloadSchema>;
export type UpdateIntegrationPayload = z.infer<typeof UpdateIntegrationPayloadSchema>;
export type SetIntegrationActivePayload = z.infer<typeof SetIntegrationActivePayloadSchema>;
export type EcoFlowBLEAuthStatus = z.infer<typeof EcoFlowBLEAuthStatusSchema>;
export type ConnectEcoFlowBLEAuthPayload = z.infer<typeof ConnectEcoFlowBLEAuthPayloadSchema>;
export type SetEcoFlowBLEAuthUserIDPayload = z.infer<typeof SetEcoFlowBLEAuthUserIDPayloadSchema>;
