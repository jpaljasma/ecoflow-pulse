import { buildOptions, loadConfig } from './lib/config.js';
import { setupShared } from './lib/setup.js';
import { ingestPublish as runIngestPublish } from './scenarios/ingest.js';
import { historyQuery as runHistoryQuery } from './scenarios/query.js';
import { wsFanout as runWSFanout } from './scenarios/ws.js';

const cfg = loadConfig(__ENV);

export const options = buildOptions(cfg);

export function setup() {
  return setupShared(cfg);
}

export function ingestPublish(data) {
  runIngestPublish(data);
}

export function historyQuery(data) {
  runHistoryQuery(data);
}

export function wsFanout(data) {
  runWSFanout(data);
}
