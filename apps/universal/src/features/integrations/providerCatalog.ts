import type { AvailableDeviceSummary } from '@/features/devices/schema';
import type { Integration } from './schema';

export const DEFAULT_PROVIDER = 'ecoflow';

export const PECRON_REGION_OPTIONS = [
  { id: 'us', label: 'US' },
  { id: 'eu', label: 'EU' },
  { id: 'cn', label: 'CN' }
] as const;

export const ANKER_SOLIX_SERVER_OPTIONS = [
  { id: 'com', label: 'COM' },
  { id: 'eu', label: 'EU' }
] as const;

export type PecronRegion = (typeof PECRON_REGION_OPTIONS)[number]['id'];
export type AnkerSolixServer = (typeof ANKER_SOLIX_SERVER_OPTIONS)[number]['id'];

export type ProviderConfigDraft = {
  pecronRegion: PecronRegion;
  ankerSolixServer: AnkerSolixServer;
  ankerSolixCountry: string;
};

export type ConnectorCopy = {
  id: string;
  title: string;
  description: string;
  icon: string;
  catalogTitle: string;
  catalogDescription: string;
  validationLabel: string;
  activationDescription: string;
  configureTitle: string;
  accessKeyLabel: string;
  accessSecretLabel: string;
  accessKeyPlaceholder: string;
  accessSecretPlaceholder: string;
  replacementAccessKeyPlaceholder: string;
  replacementAccessSecretPlaceholder: string;
  createDescription: string;
  addFallbackDescription: string;
  rotateDescription: string;
  reviewDescription: string;
  emptyStateDescription: string;
};

export const CONNECTOR_CATALOG = [
  {
    id: 'ecoflow',
    title: 'EcoFlow',
    description:
      'Connect your EcoFlow account keys, keep backup credentials inactive, and let Pulse validate provider access plus MQTT before switching the live connector.',
    icon: 'transmission-tower-export',
    catalogTitle: 'EcoFlow is available in Pulse',
    catalogDescription:
      'Native provider integration with discovery, saved credential rotation, and MQTT validation.',
    validationLabel: 'Provider + MQTT',
    activationDescription:
      'Activation runs provider discovery and MQTT checks for the EcoFlow devices already enabled on this account.',
    configureTitle: 'Configure EcoFlow',
    accessKeyLabel: 'Access Key',
    accessSecretLabel: 'Access Secret',
    accessKeyPlaceholder: 'Paste Access Key',
    accessSecretPlaceholder: 'Paste Access Secret',
    replacementAccessKeyPlaceholder: 'Paste replacement Access Key',
    replacementAccessSecretPlaceholder: 'Paste replacement Access Secret',
    createDescription:
      'Paste your EcoFlow Access Key and Access Secret to create the first configured connection.',
    addFallbackDescription: 'Store another EcoFlow credential as a fallback or future replacement.',
    rotateDescription:
      'Replace the active key material. Pulse will validate the replacement before switching the live connection.',
    reviewDescription: 'Activate this saved key as-is or rotate its secret before making it live.',
    emptyStateDescription:
      'Add your first EcoFlow key pair and Pulse will validate it before activation.'
  },
  {
    id: 'pulsemqtt',
    title: 'Pulse MQTT Emulator',
    description:
      'Use the local EcoFlow-compatible emulator for signed discovery, MQTT validation, and emulator-backed DPU-X testing without a real upstream account.',
    icon: 'server-network',
    catalogTitle: 'Pulse MQTT Emulator is available in Pulse',
    catalogDescription:
      'Local provider integration with signed REST discovery, MQTT certification, and emulator-backed quota streaming.',
    validationLabel: 'Signed REST + MQTT',
    activationDescription:
      'Activation runs emulator discovery and MQTT checks for the emulator-backed devices already enabled on this account.',
    configureTitle: 'Configure Pulse MQTT Emulator',
    accessKeyLabel: 'Access Key',
    accessSecretLabel: 'Access Secret',
    accessKeyPlaceholder: 'Paste Access Key',
    accessSecretPlaceholder: 'Paste Access Secret',
    replacementAccessKeyPlaceholder: 'Paste replacement Access Key',
    replacementAccessSecretPlaceholder: 'Paste replacement Access Secret',
    createDescription:
      'Paste the emulator Access Key and Access Secret to create the first configured local connector.',
    addFallbackDescription: 'Store another emulator credential as a fallback or future replacement.',
    rotateDescription:
      'Replace the active emulator key material. Pulse will validate the replacement before switching the live connection.',
    reviewDescription: 'Activate this saved emulator credential as-is or rotate its secret before making it live.',
    emptyStateDescription:
      'Add your first Pulse MQTT emulator key pair and Pulse will validate it before activation.'
  },
  {
    id: 'pecron',
    title: 'Pecron',
    description:
      'Connect a Pecron cloud account, select the cloud region, discover E1000LFP devices, and stream read-only telemetry through the shared Pulse pipeline.',
    icon: 'battery-sync-outline',
    catalogTitle: 'Pecron E1000LFP is available in Pulse',
    catalogDescription:
      'Unofficial Pecron cloud integration with region-aware discovery, REST snapshots, and MQTT live telemetry.',
    validationLabel: 'Cloud REST + MQTT',
    activationDescription:
      'Activation signs in to the selected Pecron region, discovers supported devices, and validates the MQTT live feed for enabled E1000LFP units.',
    configureTitle: 'Configure Pecron',
    accessKeyLabel: 'Email',
    accessSecretLabel: 'Password',
    accessKeyPlaceholder: 'Pecron account email',
    accessSecretPlaceholder: 'Pecron account password',
    replacementAccessKeyPlaceholder: 'Replacement Pecron account email',
    replacementAccessSecretPlaceholder: 'Replacement Pecron password',
    createDescription:
      'Enter the Pecron account email and password used by the Pecron app, then choose the matching cloud region.',
    addFallbackDescription: 'Store another Pecron credential as a fallback or future replacement.',
    rotateDescription:
      'Replace the active Pecron account credentials. Pulse will validate discovery and MQTT before switching the live connection.',
    reviewDescription: 'Activate this saved Pecron credential as-is or rotate its password before making it live.',
    emptyStateDescription:
      'Add your first Pecron account and Pulse will validate it before activation.'
  },
  {
    id: 'anker_solix',
    title: 'Anker SOLIX Cloud MQTT',
    description:
      'Connect an Anker SOLIX account, choose the cloud server and account country, discover supported power stations and home-battery systems, and stream read-only cloud MQTT telemetry.',
    icon: 'home-battery-outline',
    catalogTitle: 'Anker SOLIX Cloud MQTT is available in Pulse',
    catalogDescription:
      'Unofficial Anker SOLIX cloud integration for power stations and home-battery systems with REST discovery and MQTT telemetry.',
    validationLabel: 'Cloud REST + MQTT',
    activationDescription:
      'Activation signs in to the selected Anker cloud, discovers supported SOLIX systems, and validates the MQTT live feed before enabling devices.',
    configureTitle: 'Configure Anker SOLIX',
    accessKeyLabel: 'Email',
    accessSecretLabel: 'Password',
    accessKeyPlaceholder: 'Anker account email',
    accessSecretPlaceholder: 'Anker account password',
    replacementAccessKeyPlaceholder: 'Replacement Anker account email',
    replacementAccessSecretPlaceholder: 'Replacement Anker password',
    createDescription:
      'Enter the Anker account email and password used by the Anker app, then choose the cloud server and country assigned to the account.',
    addFallbackDescription: 'Store another Anker SOLIX credential as a fallback or future replacement.',
    rotateDescription:
      'Replace the active Anker SOLIX account credentials. Pulse will validate discovery and MQTT before switching the live connection.',
    reviewDescription: 'Activate this saved Anker SOLIX credential as-is or rotate its password before making it live.',
    emptyStateDescription:
      'Add your first Anker SOLIX account and Pulse will validate it before activation.'
  }
] as const satisfies readonly ConnectorCopy[];

const ANKER_SOLIX_SUPPORTED_MODELS = new Set([
  'A1722',
  'A1723',
  'A1725',
  'A1726',
  'A1727',
  'A1728',
  'A1729',
  'A1761',
  'A1763',
  'A1780',
  'A1780P',
  'A1782',
  'A1783',
  'A1790',
  'A1790P',
  'A17B1',
  'A17C0',
  'A17C1',
  'A17C2',
  'A17C3',
  'A17C5',
  'A17E1',
  'A5101',
  'A5102',
  'A5103',
  'A5220',
  'A7320',
  'AE100',
  'AX170'
]);

const ANKER_SOLIX_UNSUPPORTED_MODELS = new Set([
  'A1753',
  'A1754',
  'A1755',
  'A1762',
  'A1765',
  'A1771',
  'A1772',
  'A1781',
  'A1785',
  'A17E2',
  'AS100'
]);

export type AvailableDeviceSupport = {
  label: string;
  detail: string;
  tone: 'success' | 'warning' | 'neutral';
  enableable: boolean;
};

export function getConnectorCopy(provider: string): ConnectorCopy {
  return CONNECTOR_CATALOG.find((item) => item.id === provider) ?? CONNECTOR_CATALOG[0];
}

export function formatProviderLabel(provider: string): string {
  const catalog = CONNECTOR_CATALOG.find((item) => item.id === provider);
  if (catalog) {
    return catalog.title;
  }
  return provider
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');
}

export function createProviderConfigDraft(
  provider: string,
  config: Record<string, unknown> = {}
): ProviderConfigDraft {
  return {
    pecronRegion: provider === 'pecron' ? normalizePecronRegion(config.region) : 'us',
    ankerSolixServer:
      provider === 'anker_solix' ? normalizeAnkerSolixServer(config.server) : 'com',
    ankerSolixCountry:
      provider === 'anker_solix' ? normalizeAnkerSolixCountry(config.country) : 'US'
  };
}

export function buildProviderCredentialConfig(
  provider: string,
  draft: ProviderConfigDraft
): Record<string, unknown> {
  if (provider === 'pecron') {
    return { region: draft.pecronRegion };
  }
  if (provider === 'anker_solix') {
    return {
      server: draft.ankerSolixServer,
      country: normalizeAnkerSolixCountry(draft.ankerSolixCountry)
    };
  }
  return {};
}

export function formatIntegrationConfigSummary(integration: Integration): string {
  if (integration.provider === 'pecron') {
    return `${formatPecronRegion(normalizePecronRegion(integration.config?.region))} region`;
  }
  if (integration.provider === 'anker_solix') {
    const server = normalizeAnkerSolixServer(integration.config?.server).toUpperCase();
    const country = normalizeAnkerSolixCountry(integration.config?.country);
    return `${server} cloud, ${country} account`;
  }
  return '';
}

export function describeAvailableDeviceSupport(
  device: AvailableDeviceSummary
): AvailableDeviceSupport | null {
  if (device.provider !== 'anker_solix') {
    return null;
  }
  const status = supportStatusFromRecord(device.metadata) ?? supportStatusFromRecord(device.capabilities);
  const model = extractModelCode(device.model || device.name);
  const normalizedStatus =
    status ??
    (model && ANKER_SOLIX_SUPPORTED_MODELS.has(model)
      ? 'supported'
      : model && ANKER_SOLIX_UNSUPPORTED_MODELS.has(model)
        ? 'unsupported'
        : null);

  switch (normalizedStatus) {
    case 'supported':
      return {
        label: 'Supported',
        tone: 'success',
        enableable: true,
        detail: 'Mapped for Anker SOLIX Cloud MQTT telemetry and ready for a live MQTT probe.'
      };
    case 'companion':
      return {
        label: 'Companion telemetry',
        tone: 'warning',
        enableable: false,
        detail:
          'Parsed only when embedded in a supported home-backup system; standalone enablement is not part of V1.'
      };
    case 'partial':
      return {
        label: 'Partial support',
        tone: 'warning',
        enableable: true,
        detail: 'Basic telemetry is mapped; enablement still requires a successful live MQTT probe.'
      };
    case 'unsupported':
      return {
        label: 'Needs sample',
        tone: 'neutral',
        enableable: false,
        detail: 'Pulse discovered this SOLIX model, but needs a tested MQTT descriptor before enablement.'
      };
    default:
      return null;
  }
}

export function normalizeCountryInput(value: string): string {
  return value.replace(/[^a-z]/gi, '').slice(0, 2).toUpperCase();
}

function normalizePecronRegion(value: unknown): PecronRegion {
  const text = typeof value === 'string' ? value.trim().toLowerCase() : '';
  return PECRON_REGION_OPTIONS.some((region) => region.id === text) ? (text as PecronRegion) : 'us';
}

function formatPecronRegion(region: PecronRegion): string {
  return PECRON_REGION_OPTIONS.find((option) => option.id === region)?.label ?? 'US';
}

function normalizeAnkerSolixServer(value: unknown): AnkerSolixServer {
  const text = typeof value === 'string' ? value.trim().toLowerCase() : '';
  return ANKER_SOLIX_SERVER_OPTIONS.some((server) => server.id === text)
    ? (text as AnkerSolixServer)
    : 'com';
}

function normalizeAnkerSolixCountry(value: unknown): string {
  const text = typeof value === 'string' ? value.trim().toUpperCase() : '';
  return /^[A-Z]{2}$/.test(text) ? text : 'US';
}

function supportStatusFromRecord(
  record: Record<string, unknown> | undefined
): 'supported' | 'partial' | 'unsupported' | 'companion' | null {
  if (!record) {
    return null;
  }
  const raw =
    record.supportStatus ??
    record.support_status ??
    record.mqttSupportStatus ??
    record.mqtt_support_status ??
    record.ankerSolixSupport ??
    record.anker_solix_support ??
    record.supported;
  if (typeof raw === 'boolean') {
    return raw ? 'supported' : 'unsupported';
  }
  if (typeof raw !== 'string') {
    return null;
  }
  const text = raw.trim().toLowerCase().replace(/[\s-]+/g, '_');
  if (['supported', 'full', 'enabled', 'mapped'].includes(text)) {
    return 'supported';
  }
  if (['partial', 'basic', 'basic_mqtt', 'mqtt_basic'].includes(text)) {
    return 'partial';
  }
  if (['companion', 'embedded', 'embedded_companion'].includes(text)) {
    return 'companion';
  }
  if (['unsupported', 'needs_sample', 'needs_descriptor', 'sample_needed'].includes(text)) {
    return 'unsupported';
  }
  return null;
}

function extractModelCode(value: string): string | null {
  const match = value.toUpperCase().match(/\b(?:A\d{4}P?|A\d{2}[A-Z]\d|AX\d{3}|AE\d{3}|AS\d{3})\b/);
  return match?.[0] ?? null;
}
