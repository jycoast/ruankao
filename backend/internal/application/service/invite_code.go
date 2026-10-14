package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// 邀请码字符集：去掉易混淆的 O/0/I/1
const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type InviteCodeService struct {
	repo repository.InviteCodeRepository
}

func NewInviteCodeService(repo repository.InviteCodeRepository) *InviteCodeService {
	return &InviteCodeService{repo: repo}
}

func generateCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, v := range b {
		if i == 4 {
			sb.WriteByte('-')
		}
		sb.WriteByte(codeChars[int(v)%len(codeChars)])
	}
	return sb.String(), nil
}

func (s *InviteCodeService) Generate(count int, createdBy uint64, note string) ([]string, error) {
	if count <= 0 || count > 100 {
		return nil, errors.New("生成数量必须在 1-100 之间")
	}

	codes := make([]model.InviteCode, 0, count)
	generated := make([]string, 0, count)
	seen := map[string]struct{}{}

	for len(codes) < count {
		code, err := generateCode()
		if err != nil {
			return nil, err
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, model.InviteCode{
			Code:      code,
			CreatedBy: createdBy,
			Note:      note,
		})
		generated = append(generated, code)
	}

	if err := s.repo.BatchCreate(codes); err != nil {
		return nil, fmt.Errorf("保存失败: %w", err)
	}
	return generated, nil
}

func (s *InviteCodeService) Validate(code string) error {
	c, err := s.repo.FindByCode(code)
	if err != nil {
		return errors.New("邀请码无效")
	}
	if c.Status != 0 {
		return errors.New("邀请码已被使用")
	}
	return nil
}

func (s *InviteCodeService) UseCode(code string, userID uint64) error {
	return s.repo.MarkUsed(code, userID)
}

func (s *InviteCodeService) List(page, pageSize int) ([]model.InviteCode, int64, error) {
	return s.repo.List(page, pageSize)
}

func (s *InviteCodeService) Delete(id uint64) error {
	return s.repo.Delete(id)
}
