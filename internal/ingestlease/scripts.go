package ingestlease

import valkey "github.com/valkey-io/valkey-go"

const (
	leaseStateActive   = "active"
	leaseStateDraining = "draining"
)

var (
	leaseAcquireScript = valkey.NewLuaScriptRetryable(`
local lease_key = KEYS[1]
local session_key = KEYS[2]
local fence_key = KEYS[3]

local token = ARGV[1]
local worker_id = ARGV[2]
local ttl_ms = tonumber(ARGV[3])
local now_ms = ARGV[4]
local provider = ARGV[5]
local provider_device_id = ARGV[6]

if redis.call('EXISTS', lease_key) == 1 then
  local current_token = redis.call('HGET', lease_key, 'token') or ''
  local current_fence = tonumber(redis.call('HGET', lease_key, 'fence') or '0')
  return {0, current_fence, current_token}
end

local next_fence = redis.call('INCR', fence_key)
local fence_str = tostring(next_fence)

redis.call('HSET', lease_key,
  'token', token,
  'worker_id', worker_id,
  'fence', fence_str,
  'state', 'active',
  'updated_at', now_ms
)
redis.call('PEXPIRE', lease_key, ttl_ms)

redis.call('HSET', session_key,
  'token', token,
  'worker_id', worker_id,
  'fence', fence_str,
  'provider', provider,
  'provider_device_id', provider_device_id,
  'state', 'active',
  'updated_at', now_ms
)
redis.call('PEXPIRE', session_key, ttl_ms)

return {1, next_fence, token}
`)

	leaseRenewScript = valkey.NewLuaScriptRetryable(`
local lease_key = KEYS[1]
local session_key = KEYS[2]

local token = ARGV[1]
local worker_id = ARGV[2]
local ttl_ms = tonumber(ARGV[3])
local now_ms = ARGV[4]
local state = ARGV[5]

if redis.call('EXISTS', lease_key) == 0 then
  return {0, 0, 'missing'}
end

local current_token = redis.call('HGET', lease_key, 'token')
if current_token ~= token then
  local current_fence = tonumber(redis.call('HGET', lease_key, 'fence') or '0')
  return {0, current_fence, 'token_mismatch'}
end

local current_fence = tonumber(redis.call('HGET', lease_key, 'fence') or '0')
redis.call('HSET', lease_key,
  'worker_id', worker_id,
  'state', state,
  'updated_at', now_ms
)
redis.call('PEXPIRE', lease_key, ttl_ms)

redis.call('HSET', session_key,
  'token', token,
  'worker_id', worker_id,
  'fence', current_fence,
  'state', state,
  'updated_at', now_ms
)
redis.call('PEXPIRE', session_key, ttl_ms)

return {1, current_fence, state}
`)

	leaseReleaseScript = valkey.NewLuaScriptRetryable(`
local lease_key = KEYS[1]
local session_key = KEYS[2]

local token = ARGV[1]

if redis.call('EXISTS', lease_key) == 0 then
  return {0, 'missing'}
end

local current_token = redis.call('HGET', lease_key, 'token')
if current_token ~= token then
  return {0, 'token_mismatch'}
end

redis.call('DEL', lease_key)
redis.call('DEL', session_key)
return {1, 'released'}
`)
)
