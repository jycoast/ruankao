package service

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type KnowledgeService struct {
	kpRepo repository.KnowledgePointRepository
}

func NewKnowledgeService(kpRepo repository.KnowledgePointRepository) *KnowledgeService {
	return &KnowledgeService{kpRepo: kpRepo}
}

func (s *KnowledgeService) List(categoryID, subjectID uint64, page, pageSize int) ([]model.KnowledgePoint, int64, error) {
	return s.kpRepo.List(categoryID, subjectID, page, pageSize)
}

func (s *KnowledgeService) GetByID(id uint64) (*model.KnowledgePoint, error) {
	return s.kpRepo.FindByID(id)
}

// RelatedQuestions 返回知识点关联的真题：优先用 match_key 关键词，为空则回退用名称匹配
func (s *KnowledgeService) RelatedQuestions(id uint64, limit int) ([]model.RelatedQuestion, error) {
	kp, err := s.kpRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	var keywords []string
	if strings.TrimSpace(kp.MatchKey) != "" {
		for _, k := range strings.Split(kp.MatchKey, "|") {
			if k = strings.TrimSpace(k); k != "" {
				keywords = append(keywords, k)
			}
		}
	} else {
		keywords = []string{kp.Name}
	}
	subjectID := kp.SubjectID
	if subjectID == 0 {
		subjectID = 1
	}
	return s.kpRepo.FindRelatedQuestions(keywords, subjectID, limit)
}

// validateParent 校验上级节点合法性：必须存在、与节点同科目，且不在节点自己的
// 祖先链上。悬空/跨科目父级会让节点被 buildTree 顶成顶级节点；成环后环内节点
// 无根可达，会从树里整体消失（显示不出也管理不到）。id 传 0 表示新增场景。
func (s *KnowledgeService) validateParent(id, parentID, subjectID uint64) error {
	if parentID == 0 {
		return nil
	}
	if parentID == id {
		return repository.ErrKnowledgeCycle
	}
	parent, err := s.kpRepo.FindByID(parentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.ErrKnowledgeBadParent
		}
		return err
	}
	if parent.SubjectID != subjectID {
		return repository.ErrKnowledgeBadParent
	}
	// 沿新上级的祖先链向上走：碰到自己即成环；seen 防库中已有环导致死循环
	seen := map[uint64]bool{}
	for cur := parent; ; {
		if cur.ID == id || seen[cur.ID] {
			return repository.ErrKnowledgeCycle
		}
		seen[cur.ID] = true
		if cur.ParentID == 0 {
			return nil
		}
		next, err := s.kpRepo.FindByID(cur.ParentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 祖先链上已有悬空引用（历史脏数据）：不阻塞本次保存
				return nil
			}
			return err
		}
		cur = next
	}
}

func (s *KnowledgeService) Create(name string, categoryID, parentID, subjectID uint64, content, keyPoints, easyMistakes string, sortOrder int) (uint64, error) {
	if subjectID == 0 {
		subjectID = 1
	}
	if err := s.validateParent(0, parentID, subjectID); err != nil {
		return 0, err
	}
	kp := &model.KnowledgePoint{
		Name:         name,
		CategoryID:   categoryID,
		ParentID:     parentID,
		SubjectID:    subjectID,
		Content:      content,
		KeyPoints:    keyPoints,
		EasyMistakes: easyMistakes,
		SortOrder:    sortOrder,
		Status:       1,
	}
	if err := s.kpRepo.Create(kp); err != nil {
		return 0, err
	}
	return kp.ID, nil
}

func (s *KnowledgeService) Update(id uint64, name string, categoryID, parentID, subjectID uint64, content, keyPoints, easyMistakes string, sortOrder int) error {
	kp, err := s.kpRepo.FindByID(id)
	if err != nil {
		return err
	}
	if name != "" {
		kp.Name = name
	}
	// 编辑表单不提供 category_id，传 0 表示保留原值，避免每次编辑都清掉分类
	if categoryID != 0 {
		kp.CategoryID = categoryID
	}
	if subjectID != 0 {
		kp.SubjectID = subjectID
	}
	// 层级防线：上级只在发生变化时校验（纯内容编辑不查库）；编辑弹窗排除后代
	// 基于打开弹窗时的快照，期间他人挪动节点仍可能造成环，必须后端兜底
	if parentID != kp.ParentID {
		if err := s.validateParent(kp.ID, parentID, kp.SubjectID); err != nil {
			return err
		}
	}
	kp.ParentID = parentID
	// 内容/考点/易错点允许清空，直接覆盖
	kp.Content = content
	kp.KeyPoints = keyPoints
	kp.EasyMistakes = easyMistakes
	kp.SortOrder = sortOrder
	return s.kpRepo.Update(kp)
}

func (s *KnowledgeService) Delete(id uint64) error {
	return s.kpRepo.Delete(id)
}

// Reorder 拖拽排序：整树序列化的父级+序号批量落库（事务内），
// baseVersion 做乐观锁，返回提交后的新版本号。
func (s *KnowledgeService) Reorder(items []repository.ReorderItem, baseVersion string) (string, error) {
	return s.kpRepo.Reorder(items, baseVersion)
}
