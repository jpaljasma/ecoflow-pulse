import { bench, describe } from 'vitest';

import {
  appendLogEntry,
  createInitialLogState,
  DEFAULT_LOG_KEEP_LIMIT,
  fuzzyFilterLogEntries,
  type AdminLogEntry,
  type AppendLogState
} from '@/features/adminLogs/model';

const entries = Array.from({ length: DEFAULT_LOG_KEEP_LIMIT }, (_, index) => sampleEntry(index));

describe('admin log model performance', () => {
  bench('fuzzyFilterLogEntries over default buffered rows', () => {
    fuzzyFilterLogEntries(entries, 'quota device-12');
  });

  bench('appendLogEntry capped at default rows', () => {
    let state: AppendLogState = createInitialLogState();
    for (let index = 0; index < 1_000; index += 1) {
      state = appendLogEntry(state, sampleEntry(index), { paused: false });
    }
  });
});

function sampleEntry(index: number): AdminLogEntry {
  const status = index % 19 === 0 ? 'error' : index % 7 === 0 ? 'warning' : 'ok';
  const typeCode = index % 3 === 0 ? 'status' : 'quota';
  return {
    id: `entry-${index}`,
    ts: 1772197190000 + index,
    receivedTs: 1772197190100 + index,
    deviceId: `device-${index % 50}`,
    status,
    source: typeCode === 'status' ? 'mqtt-status' : 'mqtt',
    sourceKind: typeCode === 'status' ? 'SOURCE_KIND_MQTT_STATUS' : 'SOURCE_KIND_MQTT_QUOTA',
    typeCode,
    summary: `${typeCode} update for device ${index % 50}`,
    labels: {
      provider: 'ecoflow',
      region: `r${index % 5}`
    },
    detail: {
      deviceId: `device-${index % 50}`,
      payload: {
        soc: index % 100
      }
    }
  };
}
