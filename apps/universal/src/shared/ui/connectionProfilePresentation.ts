import type { ConnectionProfileConfig } from '@/shared/config/env';

export type ConnectionProfilePresentation = {
  iconName: 'cloud-outline' | 'cloud-sync-outline' | 'laptop';
  title: string;
  statusDescription: string;
  activeStatusDescription: string;
  detailedDescription: string;
  compactDescription: string;
};

export function describeConnectionProfileForUi(
  profile: ConnectionProfileConfig
): ConnectionProfilePresentation {
  if (profile.id === 'cloud') {
    if (profile.edge === 'local') {
      return {
        iconName: 'cloud-sync-outline',
        title: profile.label,
        statusDescription: 'Cloud data',
        activeStatusDescription: 'Selected',
        detailedDescription: 'Use the local HTTPS edge while database and realtime telemetry come from cloud.',
        compactDescription: 'Local edge with cloud data.'
      };
    }

    return {
      iconName: 'cloud-outline',
      title: profile.label,
      statusDescription: 'Hosted stack',
      activeStatusDescription: 'Selected',
      detailedDescription: 'Use the hosted HTTPS API, websocket gateway, and cloud OIDC issuer.',
      compactDescription: 'Hosted API, realtime, and auth.'
    };
  }

  return {
    iconName: 'laptop',
    title: profile.label,
    statusDescription: 'Local stack',
    activeStatusDescription: 'Selected',
    detailedDescription: 'Use your local k3d HTTPS edge and development services on this machine or LAN.',
    compactDescription: 'Local k3d edge and development services.'
  };
}
