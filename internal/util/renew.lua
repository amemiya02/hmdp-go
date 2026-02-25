-- 判断这把锁是不是我的，如果是，就重置过期时间
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], ARGV[2])
else
    return 0
end