package service

import (
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type FavoriteService struct {
	favRepo      repository.FavoriteRepository
	questionRepo repository.QuestionRepository
	articleRepo  repository.ArticleRepository
	kpRepo       repository.KnowledgePointRepository
}

func NewFavoriteService(
	favRepo repository.FavoriteRepository,
	questionRepo repository.QuestionRepository,
	articleRepo repository.ArticleRepository,
	kpRepo repository.KnowledgePointRepository,
) *FavoriteService {
	return &FavoriteService{
		favRepo:      favRepo,
		questionRepo: questionRepo,
		articleRepo:  articleRepo,
		kpRepo:       kpRepo,
	}
}

func (s *FavoriteService) Add(userID uint64, targetType int8, targetID uint64) error {
	_, err := s.favRepo.FindByUserAndTarget(userID, targetType, targetID)
	if err == nil {
		return nil
	}
	fav := &model.Favorite{
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
	}
	return s.favRepo.Create(fav)
}

func (s *FavoriteService) Remove(userID uint64, targetType int8, targetID uint64) error {
	return s.favRepo.Delete(userID, targetType, targetID)
}

// FavoriteItem 收藏列表项。三种目标（题目/论文/知识点）字段形态不同：
// 题目/论文带 Title，知识点带 Name。刻意收窄成这 3 个字段——
// 前端只消费它们（见 Favorites.vue），不再把整个实体对象透传出接口。
type FavoriteItem struct {
	ID    uint64 `json:"id"`
	Title string `json:"title,omitempty"` // target_type 1 题目 / 2 论文
	Name  string `json:"name,omitempty"`  // target_type 3 知识点
}

// resolveItem 把一条收藏解析成列表项。目标实体可能已被删除（孤儿收藏），
// 此时返回 ok=false，调用方跳过该条。
func (s *FavoriteService) resolveItem(targetType int8, targetID uint64) (FavoriteItem, bool) {
	switch targetType {
	case 1:
		q, err := s.questionRepo.FindByID(targetID)
		if err == nil {
			return FavoriteItem{ID: q.ID, Title: q.Title}, true
		}
	case 2:
		a, err := s.articleRepo.FindByID(targetID)
		if err == nil {
			return FavoriteItem{ID: a.ID, Title: a.Title}, true
		}
	case 3:
		kp, err := s.kpRepo.FindByID(targetID)
		if err == nil {
			return FavoriteItem{ID: kp.ID, Name: kp.Name}, true
		}
	}
	return FavoriteItem{}, false
}

// List 按 targetType 列出收藏。逐条 FindByID 是 N+1，由 page_size≤50 约束住
// 上限，不引入联表。孤儿收藏（实体已删）会让当前页解析后不足一页甚至空页，
// 因此向后翻页回补凑满当前页；返回行不足 page_size 即最后一页，到此为止。
// total 透传收藏总数（不是当前页条数）。
func (s *FavoriteService) List(userID uint64, targetType int8, page, pageSize int) ([]FavoriteItem, int64, error) {
	result := make([]FavoriteItem, 0, pageSize)
	var total int64
	for cur := page; ; cur++ {
		favs, t, err := s.favRepo.FindByUserID(userID, targetType, cur, pageSize)
		if err != nil {
			return nil, 0, err
		}
		if cur == page {
			total = t
		}
		for _, fav := range favs {
			item, ok := s.resolveItem(targetType, fav.TargetID)
			if !ok {
				continue // 孤儿收藏：实体已删，跳过
			}
			result = append(result, item)
			if len(result) >= pageSize {
				return result, total, nil
			}
		}
		// 本页返回行数不足 page_size → 已是最后一页，凑不满也只能到此为止
		if len(favs) < pageSize {
			return result, total, nil
		}
	}
}

func (s *FavoriteService) Exists(userID uint64, targetType int8, targetID uint64) (bool, error) {
	_, err := s.favRepo.FindByUserAndTarget(userID, targetType, targetID)
	if err != nil {
		return false, nil
	}
	return true, nil
}
