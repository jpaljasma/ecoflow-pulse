import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';
import protobuf from 'protobufjs';

import { decodeEnvelope } from '../src/live/envelopeCodec.js';

type ContractEnvelope = {
  envelopeId: string;
  envelopeVersion: number;
  deviceId: string;
  ecoflowSn: string;
  shard: number;
  shardCount: number;
  messageId: string;
  deviceTimeUnixMs: number;
  observedTimeUnixMs: number;
  ingestedTimeUnixMs: number;
  sourceKind: string;
  source: string;
  typeCode: string;
  payloadType: string;
  payloadVersion: number;
  payloadEncoding: string;
  payloadBase64: string;
  payloadUtf8: string;
  labels?: Record<string, string>;
};

type ContractFixture = {
  envelopeBase64: string;
  expected: ContractEnvelope;
  nodeMessage: Record<string, unknown>;
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '../../..');
const envelopeProtoPath = path.join(repoRoot, 'proto/pulse/envelope/v1/envelope.proto');
const contractTestTimeoutMs = 15_000;
let cachedFixture: ContractFixture | undefined;

function runFixtureCommand(args: string[] = []): string {
  return execFileSync('go', ['run', './cmd/proto-contract-fixture', ...args], {
    cwd: repoRoot,
    encoding: 'utf8'
  });
}

function loadFixture(): ContractFixture {
  cachedFixture ??= JSON.parse(runFixtureCommand()) as ContractFixture;
  return cachedFixture;
}

describe('node-go protobuf contract', () => {
  it('decodes Go-generated envelope bytes with gateway decoder', () => {
    const fixture = loadFixture();
    const bytes = Buffer.from(fixture.envelopeBase64, 'base64');
    const decoded = decodeEnvelope(bytes);
    expect(decoded).not.toBeNull();
    if (!decoded) {
      return;
    }

    expect(decoded.deviceId).toBe(fixture.expected.deviceId);
    expect(decoded.ecoflowSn).toBe(fixture.expected.ecoflowSn);
    expect(decoded.messageId).toBe(fixture.expected.messageId);
    expect(decoded.deviceTimeUnixMs).toBe(fixture.expected.deviceTimeUnixMs);
    expect(decoded.observedTimeUnixMs).toBe(fixture.expected.observedTimeUnixMs);
    expect(decoded.ingestedTimeUnixMs).toBe(fixture.expected.ingestedTimeUnixMs);
    expect(decoded.typeCode).toBe(fixture.expected.typeCode);
    expect(decoded.payloadEncoding).toBe(fixture.expected.payloadEncoding);
    expect(Buffer.from(decoded.payload).toString('base64')).toBe(fixture.expected.payloadBase64);
    expect(Buffer.from(decoded.payload).toString('utf8')).toBe(fixture.expected.payloadUtf8);
  }, contractTestTimeoutMs);

  it('round-trips Node-generated envelope bytes through Go decoder', () => {
    const fixture = loadFixture();
    const root = protobuf.loadSync(envelopeProtoPath);
    const telemetryEnvelopeType = root.lookupType('pulse.envelope.v1.TelemetryEnvelope');

    const verificationError = telemetryEnvelopeType.verify(fixture.nodeMessage);
    expect(verificationError).toBeNull();

    const message = telemetryEnvelopeType.fromObject(fixture.nodeMessage);
    const encoded = telemetryEnvelopeType.encode(message).finish();
    const decodedByGo = JSON.parse(
      runFixtureCommand(['-decode-base64', Buffer.from(encoded).toString('base64')])
    ) as ContractEnvelope;

    expect(decodedByGo).toStrictEqual(fixture.expected);
  }, contractTestTimeoutMs);
});
