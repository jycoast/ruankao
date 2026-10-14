package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type CardKeyService struct {
	repo          repository.CardKeyRepository
	membershipSvc *MembershipService
	planSvc       *PlanService
}

func NewCardKeyService(repo repository.CardKeyRepository, membershipSvc *MembershipService, planSvc *PlanService) *CardKeyService {
	return &CardKeyService{repo: repo, membershipSvc: membershipSvc, planSvc: planSvc}
}

// generateCardCode 生成 4 段卡密：XXXX-XXXX-XXXX-XXXX，
// 复用邀请码的去混淆字符集 codeChars（invite_code.go）。
func generateCardCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, v := range b {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(codeChars[int(v)%len(codeChars)])
	}
	return sb.String(), nil
}

// Generate 批量生成绑定指定套餐的卡密。plan 必须是已存在的套餐 key。
func (s *CardKeyService) Generate(count int, plan string, createdBy uint64, note string) ([]string, error) {
	if count <= 0 || count > 100 {
		return nil, errors.New("生成数量必须在 1-100 之间")
	}
	if _, ok := s.planSvc.Find(plan); !ok {
		return nil, errors.New("套餐不存在")
	}

	codes := make([]model.CardKey, 0, count)
	generated := make([]string, 0, count)
	seen := map[string]struct{}{}

	for len(codes) < count {
		code, err := generateCardCode()
		if err != nil {
			return nil, err
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, model.CardKey{
			Code:      code,
			Plan:      plan,
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

// Redeem 兑换卡密：校验有效性 → 原子占用 → 开通/续期会员。
// 返回套餐名与天数供前端提示。
func (s *CardKeyService) Redeem(code string, userID uint64) (planName string, days int, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", 0, errors.New("请输入卡密")
	}

	card, err := s.repo.FindByCode(code)
	if err != nil {
		return "", 0, errors.New("卡密无效")
	}
	if card.Status != 0 {
		return "", 0, errors.New("卡密已被使用")
	}

	plan, ok := s.planSvc.Find(card.Plan)
	if !ok {
		return "", 0, errors.New("卡密对应的套餐已下线，请联系客服")
	}

	// 先原子占用（防并发重复兑换），再发放会员。
	// MarkUsed 在 status 已非 0 时返回错误，天然幂等。
	if err := s.repo.MarkUsed(code, userID); err != nil {
		return "", 0, errors.New("卡密已被使用")
	}
	if err := s.membershipSvc.Grant(userID, plan.Key, plan.Days); err != nil {
		return "", 0, fmt.Errorf("开通会员失败: %w", err)
	}
	return plan.Name, plan.Days, nil
}

func (s *CardKeyService) List(req repository.CardKeyListReq) ([]model.CardKey, int64, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	return s.repo.List(req)
}

func (s *CardKeyService) Delete(id uint64) error {
	return s.repo.Delete(id)
}
