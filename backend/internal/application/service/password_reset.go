package service

import (
	"errors"
	"strings"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"

	"golang.org/x/crypto/bcrypt"
)

// pendingPerUser 单账号待处理申请上限。超过就静默丢弃 ——
// 限流按 IP 拦不住换 IP 重刷，没有这层的话一个账号能把后台列表刷成噪音。
const pendingPerUser = 3

type PasswordResetService struct {
	repo     repository.PasswordResetRepository
	userRepo repository.UserRepository
}

func NewPasswordResetService(repo repository.PasswordResetRepository, userRepo repository.UserRepository) *PasswordResetService {
	return &PasswordResetService{repo: repo, userRepo: userRepo}
}

// Submit 提交找回密码申请。account 可以是用户名或邮箱。
//
// 无论账号是否存在都返回 nil —— 报「用户不存在」等于送对方一个账号枚举接口。
// 账号不存在时也不建行：管理员看到的列表本身就是一份「这些账号真实存在」的名单。
func (s *PasswordResetService) Submit(account, contact, reason, clientIP string) error {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil
	}

	user, err := s.userRepo.FindByUsername(account)
	if err != nil || user == nil {
		user, err = s.userRepo.FindByEmail(account)
		if err != nil || user == nil {
			return nil
		}
	}

	// 封禁账号不该有找回入口，处理了也登不进来
	if user.Status != 1 {
		return nil
	}

	if n, err := s.repo.CountPendingByUser(user.ID); err == nil && n >= pendingPerUser {
		return nil
	}

	return s.repo.Create(&model.PasswordReset{
		UserID:   user.ID,
		Account:  truncateRunes(account, 100),
		Contact:  truncateRunes(strings.TrimSpace(contact), 100),
		Reason:   truncateRunes(strings.TrimSpace(reason), 500),
		ClientIP: clientIP,
		Status:   0,
	})
}

func (s *PasswordResetService) ListAll(page, pageSize, status int) ([]repository.PasswordResetView, int64, error) {
	return s.repo.ListAll(page, pageSize, status)
}

func (s *PasswordResetService) CountPending() (int64, error) {
	return s.repo.CountPending()
}

// Approve 管理员通过申请：直接把该账号的密码改成 newPassword，并把申请标记为已处理。
// 新密码只在响应里回给管理员一次，不落库 —— 存下来等于给自己造了一张明文密码表。
func (s *PasswordResetService) Approve(id uint64, newPassword, adminNote string) error {
	if len(newPassword) < 6 {
		return errors.New("新密码至少 6 位")
	}
	req, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("申请不存在")
	}
	if req.Status != 0 {
		return errors.New("该申请已处理")
	}
	user, err := s.userRepo.FindByID(req.UserID)
	if err != nil {
		return errors.New("用户不存在")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	return s.repo.UpdateStatus(id, 1, adminNote)
}

// Reject 忽略申请，不动密码。
func (s *PasswordResetService) Reject(id uint64, adminNote string) error {
	req, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("申请不存在")
	}
	if req.Status != 0 {
		return errors.New("该申请已处理")
	}
	return s.repo.UpdateStatus(id, 2, adminNote)
}
