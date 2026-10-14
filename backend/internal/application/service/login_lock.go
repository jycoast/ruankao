package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"software-exam/config"
	"software-exam/pkg/redisclient"

	"github.com/redis/go-redis/v9"
)

// loginLockStore abstracts Redis so unit tests can use an in-memory fake.
type loginLockStore interface {
	IncrFail(ctx context.Context, failKey, lockKey string, windowSec, maxFails int) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, bool, error)
	Del(ctx context.Context, keys ...string) error
}

// LoginLockService 登录失败锁定。
// 账号维度防单账号爆破，IP 维度防扫号；成功登录只清账号计数，不清 IP，
// 避免“穿插一次正确登录”洗白爆破 IP。
type LoginLockService struct {
	store loginLockStore
}

func NewLoginLockService() *LoginLockService {
	return &LoginLockService{}
}

func NewLoginLockServiceWithStore(store loginLockStore) *LoginLockService {
	return &LoginLockService{store: store}
}

type loginLockCfg struct {
	Enable       bool
	MaxFailsUser int
	MaxFailsIP   int
	WindowSec    int
}

func lockCfg() loginLockCfg {
	raw := config.AppConfig.AuthLock
	cfg := loginLockCfg{
		Enable:       raw.Enable,
		MaxFailsUser: raw.MaxFailsUser,
		MaxFailsIP:   raw.MaxFailsIP,
		WindowSec:    raw.WindowSec,
	}
	if cfg.MaxFailsUser <= 0 {
		cfg.MaxFailsUser = 5
	}
	if cfg.MaxFailsIP <= 0 {
		cfg.MaxFailsIP = 20
	}
	if cfg.WindowSec <= 0 {
		cfg.WindowSec = 900
	}
	return cfg
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func userFailKey(username string) string { return fmt.Sprintf("auth:fail:u:%s", username) }
func userLockKey(username string) string { return fmt.Sprintf("auth:lock:u:%s", username) }
func ipFailKey(ip string) string         { return fmt.Sprintf("auth:fail:ip:%s", ip) }
func ipLockKey(ip string) string         { return fmt.Sprintf("auth:lock:ip:%s", ip) }

// recordFailScript 原子递增失败次数；达到阈值时写入 lock key。
// KEYS[1]=fail key, KEYS[2]=lock key
// ARGV[1]=window sec, ARGV[2]=max fails
var recordFailScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
if n >= tonumber(ARGV[2]) then
  redis.call('SET', KEYS[2], '1', 'EX', ARGV[1])
end
return n
`)

type redisLoginLockStore struct {
	rdb *redis.Client
}

func (s redisLoginLockStore) IncrFail(ctx context.Context, failKey, lockKey string, windowSec, maxFails int) (int64, error) {
	return recordFailScript.Run(ctx, s.rdb, []string{failKey, lockKey}, windowSec, maxFails).Int64()
}

func (s redisLoginLockStore) TTL(ctx context.Context, key string) (time.Duration, bool, error) {
	n, err := s.rdb.Exists(ctx, key).Result()
	if err != nil {
		return 0, false, err
	}
	if n == 0 {
		return 0, false, nil
	}
	ttl, err := s.rdb.TTL(ctx, key).Result()
	if err != nil {
		return 0, true, err
	}
	return ttl, true, nil
}

func (s redisLoginLockStore) Del(ctx context.Context, keys ...string) error {
	return s.rdb.Del(ctx, keys...).Err()
}

func (s *LoginLockService) resolveStore() loginLockStore {
	if s.store != nil {
		return s.store
	}
	if redisclient.Client == nil {
		return nil
	}
	return redisLoginLockStore{rdb: redisclient.Client}
}

// Check 登录前检查是否已被锁定。locked=true 时 retryAfter 为剩余秒数（至少 1）。
func (s *LoginLockService) Check(username, ip string) (locked bool, retryAfter int) {
	cfg := lockCfg()
	if !cfg.Enable {
		return false, 0
	}
	store := s.resolveStore()
	if store == nil {
		return false, 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	user := normalizeUsername(username)
	if user != "" {
		if ra, ok := ttlSeconds(ctx, store, userLockKey(user)); ok {
			return true, ra
		}
	}
	if ip != "" {
		if ra, ok := ttlSeconds(ctx, store, ipLockKey(ip)); ok {
			return true, ra
		}
	}
	return false, 0
}

// RecordFailure 记录一次失败；若刚触发锁定，返回 justLocked=true 与 retryAfter。
func (s *LoginLockService) RecordFailure(username, ip string) (justLocked bool, retryAfter int) {
	cfg := lockCfg()
	if !cfg.Enable {
		return false, 0
	}
	store := s.resolveStore()
	if store == nil {
		return false, 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	user := normalizeUsername(username)

	if user != "" {
		n, err := store.IncrFail(ctx, userFailKey(user), userLockKey(user), cfg.WindowSec, cfg.MaxFailsUser)
		if err != nil {
			log.Printf("[auth-lock] record user fail error: %v", err)
		} else if n >= int64(cfg.MaxFailsUser) {
			justLocked = true
			retryAfter = cfg.WindowSec
			if ra, ok := ttlSeconds(ctx, store, userLockKey(user)); ok {
				retryAfter = ra
			}
			log.Printf("[auth-lock] user locked username=%s fails=%d", user, n)
		}
	}

	if ip != "" {
		n, err := store.IncrFail(ctx, ipFailKey(ip), ipLockKey(ip), cfg.WindowSec, cfg.MaxFailsIP)
		if err != nil {
			log.Printf("[auth-lock] record ip fail error: %v", err)
		} else if n >= int64(cfg.MaxFailsIP) {
			justLocked = true
			ra := cfg.WindowSec
			if sec, ok := ttlSeconds(ctx, store, ipLockKey(ip)); ok {
				ra = sec
			}
			if ra > retryAfter {
				retryAfter = ra
			}
			log.Printf("[auth-lock] ip locked ip=%s fails=%d", ip, n)
		}
	}

	if justLocked && retryAfter <= 0 {
		retryAfter = cfg.WindowSec
	}
	return justLocked, retryAfter
}

// ClearUser 登录成功后清除账号维度的失败计数与锁。不清 IP 计数。
func (s *LoginLockService) ClearUser(username string) {
	cfg := lockCfg()
	if !cfg.Enable {
		return
	}
	store := s.resolveStore()
	if store == nil {
		return
	}
	user := normalizeUsername(username)
	if user == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := store.Del(ctx, userFailKey(user), userLockKey(user)); err != nil {
		log.Printf("[auth-lock] clear user error: %v", err)
	}
}

func ttlSeconds(ctx context.Context, store loginLockStore, key string) (int, bool) {
	ttl, exists, err := store.TTL(ctx, key)
	if err != nil || !exists {
		if err != nil {
			return 0, false
		}
		return 0, false
	}
	sec := int(ttl.Seconds())
	if sec <= 0 {
		return 1, true
	}
	return sec, true
}
