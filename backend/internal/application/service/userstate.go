package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"software-exam/internal/domain/repository"
	"software-exam/pkg/redisclient"

	"gorm.io/gorm"
)

// UserState 用户的实时权限态，来源恒为数据库。
type UserState struct {
	Role   int8
	Status int8
}

// Banned 账号是否已被禁用（status: 1=正常，其余为禁用）。
func (s UserState) Banned() bool { return s.Status != 1 }

// UserStateService 为鉴权中间件提供 status/role 的实时查询。
//
// 存在的理由：JWT 签发后 72 小时内不可撤销，若只信 token 里的 role，
// 封号与降权在 token 过期前都不生效。故每个请求都要以数据库为准，
// 用 Redis 缓存 60s 把成本压到可接受。
type UserStateService struct {
	userRepo repository.UserRepository
}

func NewUserStateService(userRepo repository.UserRepository) *UserStateService {
	return &UserStateService{userRepo: userRepo}
}

const (
	userStateCacheTTL = 60 * time.Second
	userStateKeyFmt   = "us:%d"
)

// ErrUserNotFound 用户在库中不存在（已被删除等），调用方应拒绝该请求。
var ErrUserNotFound = errors.New("用户不存在")

// Get 返回用户当前的 role/status。
// 数据库故障时返回 error，由调用方决定降级策略。
func (s *UserStateService) Get(ctx context.Context, userID uint64) (UserState, error) {
	if st, ok := s.fromCache(ctx, userID); ok {
		return st, nil
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UserState{}, ErrUserNotFound
		}
		return UserState{}, err
	}

	st := UserState{Role: user.Role, Status: user.Status}
	s.toCache(ctx, userID, st)
	return st, nil
}

// Invalidate 清除缓存，令封禁/降权立即生效。
func (s *UserStateService) Invalidate(userID uint64) {
	rdb := redisclient.Client
	if rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := rdb.Del(ctx, fmt.Sprintf(userStateKeyFmt, userID)).Err(); err != nil {
		// 未清掉最多延迟 60s 生效，不阻断管理员操作，但要留痕
		log.Printf("[userstate] invalidate user %d failed: %v", userID, err)
	}
}

func (s *UserStateService) fromCache(ctx context.Context, userID uint64) (UserState, bool) {
	rdb := redisclient.Client
	if rdb == nil {
		return UserState{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	raw, err := rdb.Get(ctx, fmt.Sprintf(userStateKeyFmt, userID)).Result()
	if err != nil {
		return UserState{}, false
	}
	role, status, ok := parseUserState(raw)
	if !ok {
		return UserState{}, false
	}
	return UserState{Role: role, Status: status}, true
}

func (s *UserStateService) toCache(ctx context.Context, userID uint64, st UserState) {
	rdb := redisclient.Client
	if rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	rdb.Set(ctx, fmt.Sprintf(userStateKeyFmt, userID),
		fmt.Sprintf("%d:%d", st.Role, st.Status), userStateCacheTTL)
}

func parseUserState(raw string) (role, status int8, ok bool) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	r, err := strconv.ParseInt(parts[0], 10, 8)
	if err != nil {
		return 0, 0, false
	}
	st, err := strconv.ParseInt(parts[1], 10, 8)
	if err != nil {
		return 0, 0, false
	}
	return int8(r), int8(st), true
}
