import { describe, expect, it } from 'vitest';

import {
  appendLogEntry,
  buildSubscribeFilters,
  createInitialLogState,
  fuzzyFilterLogEntries,
  isGlobalAdmin,
  redactEntryForCopy,
  resumePending,
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
        source: 'mqtt',
        typeCode: 'quota'
      })
    ).toEqual({
      deviceIds: ['dev-1', 'dev-2'],
      statuses: ['ok'],
      sources: ['mqtt'],
      typeCodes: ['quota']
    });
  });

  it('keeps live rows stable while paused and flushes pending rows on resume', () => {
    const first = sampleEntry({ id: 'first' });
    const second = sampleEntry({ id: 'second' });
    const paused = appendLogEntry({ ...createInitialLogState(), entries: [first] }, second, {
      paused: true
    });

    expect(paused.entries.map((entry) => entry.id)).toEqual(['first']);
    expect(paused.pendingCount).toBe(1);
    expect(resumePending(paused).entries.map((entry) => entry.id)).toEqual(['second', 'first']);
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
    const copied = redactEntryForCopy(sampleEntry({
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
    }));

    expect(JSON.stringify(copied)).not.toContain('owner@example.invalid');
    expect(JSON.stringify(copied)).not.toContain('SERIAL-1');
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
