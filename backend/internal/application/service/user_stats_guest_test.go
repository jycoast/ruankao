package service

import (
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"

	"gorm.io/gorm"
)

// stubUserProfileRepo:FindByUserID 恒返回未找到,Create 记录调用次数与入参。
type stubUserProfileRepo struct {
	createCalls int
	created     []*model.UserProfile
}

var _ repository.UserProfileRepository = (*stubUserProfileRepo)(nil)

func (s *stubUserProfileRepo) Create(profile *model.UserProfile) error {
	s.createCalls++
	s.created = append(s.created, profile)
	return nil
}

func (s *stubUserProfileRepo) FindByUserID(userID uint64) (*model.UserProfile, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUserProfileRepo) Update(profile *model.UserProfile) error {
	return nil
}

// 虚拟游客(user_id=0)GetStats:返回内存零 profile,绝不懒建 user_profiles 行(游客不落任何 DB 行)。
func TestUserService_GetStats_GuestSkipsCreate(t *testing.T) {
	repo := &stubUserProfileRepo{}
	s := &UserService{profileRepo: repo}

	p, err := s.GetStats(0)
	if err != nil {
		t.Fatalf("GetStats(0): %v", err)
	}
	if p.UserID != 0 {
		t.Fatalf("profile.UserID = %d, want 0", p.UserID)
	}
	if p.TotalPractice != 0 || p.TotalCorrect != 0 || p.TotalTime != 0 || p.StreakDays != 0 {
		t.Fatalf("游客零 profile 应为全零: %+v", p)
	}
	if repo.createCalls != 0 {
		t.Fatalf("游客 GetStats 不应 Create,got %d calls", repo.createCalls)
	}
}

// 真用户(user_id!=0)GetStats:无 profile 行时懒建一次(防回归)。
func TestUserService_GetStats_RealUserLazyCreate(t *testing.T) {
	repo := &stubUserProfileRepo{}
	s := &UserService{profileRepo: repo}

	p, err := s.GetStats(7)
	if err != nil {
		t.Fatalf("GetStats(7): %v", err)
	}
	if p.UserID != 7 {
		t.Fatalf("profile.UserID = %d, want 7", p.UserID)
	}
	if repo.createCalls != 1 {
		t.Fatalf("真用户 GetStats 应懒建 Create 一次,got %d calls", repo.createCalls)
	}
	if len(repo.created) != 1 || repo.created[0].UserID != 7 {
		t.Fatalf("懒建行 UserID = %v, want 7", repo.created)
	}
}
