import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const ingestAccepted = new Counter('ingest_accepted_total');

export function ingestPublish(data) {
  const timestamp = Date.now();
  const phase = (timestamp + __VU + __ITER) % 10;
  const payload = {
    device_id: data.device.id,
    serial_number: data.device.serialNumber,
    observed_unix_ms: timestamp,
    message_id: `k6-${__VU}-${__ITER}-${timestamp}`,
    metrics: {
      soc: 48 + phase,
      pv_w: 140 + phase * 15,
      load_w: 110 + phase * 8,
      ac_w: 30 + phase,
      dc_w: 16 + phase,
      battery_in_w: 175 + phase * 12,
      battery_out_w: 70 + phase * 4,
      temp_c: 22 + phase * 0.5
    }
  };

  const response = http.post(data.config.ingestUrl, JSON.stringify(payload), {
    headers: data.config.headers,
    timeout: data.config.requestTimeout
  });

  const accepted = check(response, {
    'ingest response status is 202': (res) => res.status === 202
  });
  if (accepted) {
    ingestAccepted.add(1);
  }
}
