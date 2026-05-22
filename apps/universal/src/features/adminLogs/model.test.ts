import { describe, expect, it } from 'vitest';

import {
  appendLogEntry,
  buildSubscribeFilters,
  buildAdminLogsRouteParams,
  createInitialLogState,
  DEFAULT_LOG_KEEP_LIMIT,
  fuzzyFilterLogEntries,
  isGlobalAdmin,
  mergeAdminLogFilterOptions,
  redactEntryForCopy,
  resolveAdminLogsRouteState,
  resetLogState,
  resumePending,
  toggleExclusiveStatusFilter,
  ADMIN_LOG_TYPE_FILTER_OPTIONS,
  type AdminLogEntry
} from '@/features/adminLogs/model';

describe('admin log model', () => {
  it('identifies global admin roles case-insensitively', () => {
    expect(isGlobalAdmin(['viewer'])).toBe(false);
    expect(isGlobalAdmin(['viewer', 'Admin'])).toBe(true);
  });

  it('builds websocket subscribe filters from resolved typeahead selections', () => {
    expect(
      buildSubscribeFilters({
        selectedOptions: [
          { kind: 'serial', id: 'dev-1', label: 'SERIAL-1', secondaryLabel: 'Garage', deviceIds: ['dev-1'] },
          { kind: 'user', id: 'usr-1', label: 'owner@example.invalid', secondaryLabel: 'Owner', deviceIds: ['dev-1', 'dev-2'] }
        ],
        statuses: ['ok'],
        provider: 'ecoflow',
        source: 'mqtt',
        deviceIds: ['dev-route'],
        typeCodes: ['quota', 'quota']
      })
    ).toEqual({
      deviceIds: ['dev-route', 'dev-1', 'dev-2'],
      statuses: ['ok'],
      providers: ['ecoflow'],
      sources: ['mqtt'],
      typeCodes: ['quota']
    });
  });

  it('maps log type presets to concrete MQTT type codes', () => {
    expect(ADMIN_LOG_TYPE_FILTER_OPTIONS.find((option) => option.value === 'status')?.typeCodes).toEqual([
      'pdStatus',
      'invStatus',
      'emsStatus',
      'bmsStatus',
      'mpptStatus'
    ]);
    expect(ADMIN_LOG_TYPE_FILTER_OPTIONS.find((option) => option.value === 'info')?.typeCodes).toEqual([
      'kitInfo',
      'bms_kitInfo'
    ]);
  });

  it('toggles status as an exclusive filter', () => {
    expect(toggleExclusiveStatusFilter([], 'ok')).toEqual(['ok']);
    expect(toggleExclusiveStatusFilter(['ok'], 'warning')).toEqual(['warning']);
    expect(toggleExclusiveStatusFilter(['warning'], 'warning')).toEqual([]);
    expect(toggleExclusiveStatusFilter(['ok', 'error'], 'warning')).toEqual(['warning']);
  });

  it('keeps Logs route state limited to canonical UUID device ids', () => {
    const deviceId = '019cab9d-bcab-75c0-9c02-db3ae1105d61';

    expect(resolveAdminLogsRouteState({ deviceId })).toEqual({ deviceId });
    expect(resolveAdminLogsRouteState({ device: deviceId })).toEqual({ deviceId });
    expect(resolveAdminLogsRouteState({ deviceId: 'DEMO-SERIAL' })).toEqual({});
    expect(buildAdminLogsRouteParams({ deviceId })).toEqual({ deviceId });
    expect(buildAdminLogsRouteParams({ deviceId: 'DEMO-SERIAL' })).toEqual({});
  });

  it('merges duplicate filter options without losing device ids', () => {
    expect(
      mergeAdminLogFilterOptions([
        {
          kind: 'user',
          id: 'user-1',
          label: 'owner@example.invalid',
          secondaryLabel: 'Owner',
          deviceIds: ['dev-1']
        },
        {
          kind: 'user',
          id: 'user-1',
          label: 'owner@example.invalid',
          secondaryLabel: 'Owner',
          deviceIds: ['dev-2', 'dev-1']
        },
        {
          kind: 'device',
          id: 'dev-1',
          label: 'Garage',
          secondaryLabel: 'UUID dev-1',
          deviceIds: ['dev-1']
        }
      ])
    ).toEqual([
      {
        kind: 'user',
        id: 'user-1',
        label: 'owner@example.invalid',
        secondaryLabel: 'Owner',
        deviceIds: ['dev-1', 'dev-2']
      },
      {
        kind: 'device',
        id: 'dev-1',
        label: 'Garage',
        secondaryLabel: 'UUID dev-1',
        deviceIds: ['dev-1']
      }
    ]);
  });

  it('keeps live rows stable while paused and flushes pending rows on resume', () => {
    const first = sampleEntry({ id: 'first' });
    const second = sampleEntry({ id: 'second' });
    const state = appendLogEntry(createInitialLogState(), first, { paused: false });
    const paused = appendLogEntry(state, second, {
      paused: true
    });

    expect(paused.entries.map((entry) => entry.id)).toEqual(['first']);
    expect(paused.pendingCount).toBe(1);
    expect(resumePending(paused).entries.map((entry) => entry.id)).toEqual(['second', 'first']);
  });

  it('keeps only the default visible row limit', () => {
    let state = createInitialLogState();

    for (let index = 0; index < DEFAULT_LOG_KEEP_LIMIT + 10; index += 1) {
      state = appendLogEntry(state, sampleEntry({ id: `entry-${index}` }), { paused: false });
    }

    expect(state.entries).toHaveLength(DEFAULT_LOG_KEEP_LIMIT);
    expect(state.entries[0]?.id).toBe(`entry-${DEFAULT_LOG_KEEP_LIMIT + 9}`);
    expect(state.entries.at(-1)?.id).toBe('entry-10');
  });

  it('replaces stale duplicate rows when provider message ids repeat', () => {
    let state = createInitialLogState();

    state = appendLogEntry(state, sampleEntry({ id: 'duplicate', summary: 'older frame' }), { paused: false });
    state = appendLogEntry(state, sampleEntry({ id: 'duplicate', summary: 'newer frame' }), { paused: false });

    expect(state.entries.map((entry) => entry.id)).toEqual(['duplicate']);
    expect(state.entries[0]?.summary).toBe('newer frame');
  });

  it('replaces stale duplicates in the pending paused buffer', () => {
    let state = appendLogEntry(
      createInitialLogState(),
      sampleEntry({ id: 'duplicate', summary: 'visible older frame' }),
      { paused: false }
    );

    state = appendLogEntry(state, sampleEntry({ id: 'duplicate', summary: 'older pending' }), { paused: true });
    state = appendLogEntry(state, sampleEntry({ id: 'duplicate', summary: 'newer pending' }), { paused: true });

    expect(state.entries.map((entry) => entry.summary)).toEqual(['visible older frame']);
    expect(state.pending).toHaveLength(1);
    expect(state.pendingCount).toBe(1);
    expect(resumePending(state).entries.map((entry) => entry.summary)).toEqual(['newer pending']);
  });

  it('bumps display row identity after clearing the buffer', () => {
    let state = appendLogEntry(createInitialLogState(), sampleEntry({ id: 'same' }), { paused: false });
    const firstRowKey = state.entries[0]?.rowKey;

    state = appendLogEntry(resetLogState(state), sampleEntry({ id: 'same' }), { paused: false });

    expect(state.entries[0]?.rowKey).not.toBe(firstRowKey);
  });

  it('filters buffered rows with a local freetext match', () => {
    expect(
      fuzzyFilterLogEntries([
        sampleEntry({ id: 'a', summary: 'quota update' }),
        sampleEntry({ id: 'b', summary: 'status heartbeat', source: 'mqtt-status' })
      ], 'heart')
    ).toEqual([expect.objectContaining({ id: 'b' })]);
  });

  it('matches freetext tokens across indexed log fields', () => {
    expect(
      fuzzyFilterLogEntries([
        sampleEntry({ id: 'a', deviceId: 'device-alpha', summary: 'quota update' }),
        sampleEntry({ id: 'b', deviceId: 'device-beta', summary: 'quota update' })
      ], 'quota alpha')
    ).toEqual([expect.objectContaining({ id: 'a' })]);
  });

  it('redacts sensitive fields before copy', () => {
    const buffered = appendLogEntry(
      createInitialLogState(),
      sampleEntry({
        detail: {
          email: 'owner@example.invalid',
          nested: {
            serialNumber: 'SERIAL-1',
            soc: 54
          }
        },
        labels: {
          provider: 'ecoflow',
          providerDeviceId: 'SERIAL-1'
        }
      }),
      { paused: false }
    ).entries[0]!;
    const copied = redactEntryForCopy(buffered);

    expect(JSON.stringify(copied)).not.toContain('owner@example.invalid');
    expect(JSON.stringify(copied)).not.toContain('SERIAL-1');
    expect(JSON.stringify(copied)).not.toContain('rowKey');
    expect(copied.detail).toMatchObject({ nested: { soc: 54 } });
  });
});

function sampleEntry(overrides: Partial<AdminLogEntry> = {}): AdminLogEntry {
  return {
    id: 'entry-1',
    ts: 1772197190000,
    receivedTs: 1772197190100,
    deviceId: 'dev-1',
    status: 'ok',
    source: 'mqtt',
    sourceKind: 'SOURCE_KIND_MQTT_QUOTA',
    typeCode: 'quota',
    summary: 'quota update',
    labels: { provider: 'ecoflow' },
    detail: { deviceId: 'dev-1' },
    ...overrides
  };
}
