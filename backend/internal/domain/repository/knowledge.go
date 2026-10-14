package repository

import (
	"errors"
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KnowledgePointRepository interface {
	Create(kp *model.KnowledgePoint) error
	FindByID(id uint64) (*model.KnowledgePoint, error)
	Update(kp *model.KnowledgePoint) error
	Delete(id uint64) error
	List(categoryID, subjectID uint64, page, pageSize int) ([]model.KnowledgePoint, int64, error)
	FindRelatedQuestions(keywords []string, subjectID uint64, limit int) ([]model.RelatedQuestion, error)
	// Reorder 事务内批量更新父级与序号（拖拽排序），返回提交后的新版本号。
	// baseVersion 为提交方加载列表时的 MAX(updated_at)，用于乐观锁防多管理员并发覆写。
	Reorder(items []ReorderItem, baseVersion string) (string, error)
}

// 拖拽排序整树提交的错误分类：handler 据此映射不同的响应码/文案
var (
	// ErrReorderConflict 提交基于的树快照已过期（期间被其他人改过），拒绝整树覆写
	ErrReorderConflict = errors.New("knowledge tree changed since snapshot")
	// ErrReorderBadParent 排序数据里出现了不在本批提交中的父节点（跨科目/悬空父级）
	ErrReorderBadParent = errors.New("reorder parent not in batch")
	// ErrReorderCycle 排序数据存在父子循环引用
	ErrReorderCycle = errors.New("reorder cycle detected")
	// ErrReorderIncomplete 批量里出现了「要改挂到顶级，但其当前库里的父级不在本批」
	// 的节点——说明提交方拿到的是被截断的列表快照（如分页上限截掉尾部节点），
	// 按它落库会把子节点的 parent_id 永久写成 0
	ErrReorderIncomplete = errors.New("reorder batch incomplete (truncated snapshot)")
	// ErrKnowledgeHasChildren 节点下仍有子节点，直接删会让子节点 parent_id 悬空
	// （buildTree 对悬空父级的处理是顶成顶级节点，层级凭空变形）
	ErrKnowledgeHasChildren = errors.New("knowledge node has children")
	// ErrKnowledgeBadParent 上级节点无效（不存在或跨科目）
	ErrKnowledgeBadParent = errors.New("knowledge parent invalid")
	// ErrKnowledgeCycle 上级选择会造成层级循环（环内节点在 buildTree 中无根可达，
	// 会从树里整体消失、再也无法管理）
	ErrKnowledgeCycle = errors.New("knowledge parent cycle")
)

type KnowledgePointRepositoryImpl struct {
	db *gorm.DB
}

func NewKnowledgePointRepository(db *gorm.DB) KnowledgePointRepository {
	return &KnowledgePointRepositoryImpl{db: db}
}

func (r *KnowledgePointRepositoryImpl) Create(kp *model.KnowledgePoint) error {
	return r.db.Create(kp).Error
}

func (r *KnowledgePointRepositoryImpl) FindByID(id uint64) (*model.KnowledgePoint, error) {
	var kp model.KnowledgePoint
	err := r.db.First(&kp, id).Error
	return &kp, err
}

func (r *KnowledgePointRepositoryImpl) Update(kp *model.KnowledgePoint) error {
	return r.db.Save(kp).Error
}

func (r *KnowledgePointRepositoryImpl) Delete(id uint64) error {
	// 子节点防线：前端 delete 前的 childCount 检查基于页面快照，期间他人新增
	// 子项会漏检；库里直接放行悬空会把这些子节点永久顶成顶级节点
	var children int64
	if err := r.db.Model(&model.KnowledgePoint{}).Where("parent_id = ?", id).Count(&children).Error; err != nil {
		return err
	}
	if children > 0 {
		return ErrKnowledgeHasChildren
	}
	return r.db.Delete(&model.KnowledgePoint{}, id).Error
}

// Reorder 事务内批量更新每个节点的父级与同级序号。一次拖拽提交整棵树的序列化结果，
// 部分失败时整体回滚，避免出现「拖了一部分、落了一部分」的中间态。
//
// 并发保护：前端提交时带上加载列表时的 MAX(updated_at)（baseVersion，RFC3339）。
// 期间任何一行被其他管理员改过（updated_at 更大）即视为冲突、整体拒绝——
// 否则旧快照整树覆写会把别人刚整理好的层级悄悄改回去。
func (r *KnowledgePointRepositoryImpl) Reorder(items []ReorderItem, baseVersion string) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	ids := make([]uint64, 0, len(items))
	parents := make(map[uint64]uint64, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
		parents[it.ID] = it.ParentID
	}
	// 父级必须是 0 或本批内的节点：防止把节点挂到其他科目/已不存在的父级上
	for _, p := range parents {
		if p != 0 {
			if _, ok := parents[p]; !ok {
				return "", ErrReorderBadParent
			}
		}
	}
	// 环检测：沿 parent 链向上走，重复访问即成环（非 0 父级已保证在本批内，链必终止）
	for id := range parents {
		seen := make(map[uint64]bool)
		for cur := id; cur != 0; {
			if seen[cur] {
				return "", ErrReorderCycle
			}
			seen[cur] = true
			cur = parents[cur]
		}
	}
	var cutoff time.Time
	if baseVersion != "" {
		t, err := time.Parse(time.RFC3339, baseVersion)
		if err != nil {
			return "", err
		}
		cutoff = t
	}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 锁定读一次性取回当前值：既做版本冲突比对，也做差量更新比对。
		// 版本比对两侧都经过同一套 GORM 扫描/JSON 序列化管线，时区标注一致，
		// 直接比较时刻是安全的。
		var rows []struct {
			ID        uint64    `gorm:"column:id"`
			ParentID  uint64    `gorm:"column:parent_id"`
			SortOrder int       `gorm:"column:sort_order"`
			UpdatedAt time.Time `gorm:"column:updated_at"`
		}
		if err := tx.Model(&model.KnowledgePoint{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", ids).
			Select("id, parent_id, sort_order, updated_at").
			Scan(&rows).Error; err != nil {
			return err
		}
		if !cutoff.IsZero() {
			for _, row := range rows {
				if row.UpdatedAt.After(cutoff) {
					return ErrReorderConflict
				}
			}
		}
		// 差量更新：只写真正变化的行。整树提交约 200 节点，一次拖拽实际只动
		// 几个——逐条全量写在高延迟链路（本机后端连远程库）上会拖到前端超时
		wanted := make(map[uint64]ReorderItem, len(items))
		for _, it := range items {
			wanted[it.ID] = it
		}
		for _, row := range rows {
			w := wanted[row.ID]
			if w.ParentID == row.ParentID && w.SortOrder == row.SortOrder {
				continue
			}
			// 截断快照防线：改挂到顶级的节点，其当前库里的父级必须也在本批
			// （正常拖拽中父节点就在同一棵树里）。父级不在批 = 前端快照被截断，
			// 此时把该节点写成顶级会把层级破坏永久化，整体拒绝。
			if w.ParentID == 0 && row.ParentID != 0 {
				if _, ok := parents[row.ParentID]; !ok {
					return ErrReorderIncomplete
				}
			}
			if err := tx.Model(&model.KnowledgePoint{}).
				Where("id = ?", row.ID).
				Updates(map[string]interface{}{"parent_id": w.ParentID, "sort_order": w.SortOrder}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	// 返回提交后的新版本号，前端记下供下次提交做冲突比对
	var latest time.Time
	if err := r.db.Raw("SELECT MAX(updated_at) FROM knowledge_points WHERE id IN ?", ids).Scan(&latest).Error; err != nil {
		return "", err
	}
	return latest.Format(time.RFC3339), nil
}

// FindRelatedQuestions 按关键词 LIKE 匹配 questions.knowledge_point，返回精简真题列表
func (r *KnowledgePointRepositoryImpl) FindRelatedQuestions(keywords []string, subjectID uint64, limit int) ([]model.RelatedQuestion, error) {
	var list []model.RelatedQuestion
	if len(keywords) == 0 {
		return list, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := r.db.Model(&model.Question{}).
		Select("id, title, question_type, year").
		Where("subject_id = ? AND status = 1", subjectID)

	orDB := r.db
	for i, kw := range keywords {
		if i == 0 {
			orDB = orDB.Where("knowledge_point LIKE ?", "%"+kw+"%")
		} else {
			orDB = orDB.Or("knowledge_point LIKE ?", "%"+kw+"%")
		}
	}
	err := query.Where(orDB).Order("year DESC, id DESC").Limit(limit).Scan(&list).Error
	return list, err
}

func (r *KnowledgePointRepositoryImpl) List(categoryID, subjectID uint64, page, pageSize int) ([]model.KnowledgePoint, int64, error) {
	var kps []model.KnowledgePoint
	var total int64
	query := r.db.Where("status = 1")
	if categoryID > 0 {
		query = query.Where("category_id = ?", categoryID)
	}
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	query.Model(&model.KnowledgePoint{}).Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("sort_order ASC, id ASC").Find(&kps).Error
	return kps, total, err
}

type ArticleListReq struct {
	ArticleType int8
	SubjectID   uint64
	Page        int
	PageSize    int
	Keyword     string // 模糊匹配标题
	Category    string // 精确匹配分类（为空时不过滤）
}
