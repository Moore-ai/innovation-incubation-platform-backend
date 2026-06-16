---@diagnostic disable: undefined-global
-- 令牌桶算法 Lua 脚本
-- KEYS[1]: 令牌计数器 key
-- KEYS[2]: 上次补充时间 key
-- ARGV[1]: 当前时间戳（毫秒）
-- ARGV[2]: 桶容量（max_requests_per_minute）
-- ARGV[3]: TTL（毫秒，60000）
-- 返回: 1 允许, 0 拒绝

local key = KEYS[1]
local ts_key = KEYS[2]
local now = tonumber(ARGV[1])
local cap = tonumber(ARGV[2])
local ttl_ms = tonumber(ARGV[3])

local tokens = redis.call("GET", key)
if not tokens then
	redis.call("SET", key, cap - 1, "PX", ttl_ms)
	redis.call("SET", ts_key, now, "PX", ttl_ms)
	return 1
end

local last = redis.call("GET", ts_key)
if last then
	local elapsed = now - tonumber(last)
	local refill = math.floor(elapsed / 60000 * cap)
	if refill > 0 then
		tokens = tonumber(tokens) + refill
		if tokens > cap then tokens = cap end
		redis.call("SET", ts_key, now, "PX", ttl_ms)
	end
end

if tonumber(tokens) > 0 then
	redis.call("DECR", key)
	redis.call("PEXPIRE", key, ttl_ms)
	return 1
end
return 0
