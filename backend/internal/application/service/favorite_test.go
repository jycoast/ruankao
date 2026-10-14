package service

import (
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// ---- 收藏服务的桩：favRepo 按分页返回收藏行，三个实体 repo 按 id 解析。----

type stubFavRepo struct {
	favs       []model.Favorite
	total      int64
	calls      int // FindByUserID 调用次数：回补测试要断言翻页次数
	gotTarget  int8
	gotPage    int
	gotPageSz  int
}

func (s *stubFavRepo) Create(*model.Favorite) error       { panic("unused") }
func (s *stubFavRepo) Delete(uint64, int8, uint64) error  { panic("unused") }
func (s *stubFavRepo) FindByUserAndTarget(uint64, int8, uint64) (*model.Favorite, error) {
	panic("unused")
}
func (s *stubFavRepo) FindByUserID(userID uint64, targetType int8, page, pageSize int) ([]model.Favorite, int64, error) {
	s.calls++
	s.gotTarget, s.gotPage, s.gotPageSz = targetType, page, pageSize
	// 模拟真实 repo 的分页契约：只返回当前页切片，服务层才会只解析这一页
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start > len(s.favs) {
		start = len(s.favs)
	}
	end := start + pageSize
	if end > len(s.favs) {
		end = len(s.favs)
	}
	return s.favs[start:end], s.total, nil
}

type stubFavQuestionRepo struct {
	byID  map[uint64]*model.Question
	calls int
}

func (s *stubFavQuestionRepo) FindByID(id uint64) (*model.Question, error) {
	s.calls++
	if q, ok := s.byID[id]; ok {
		return q, nil
	}
	return nil, errNotFound
}

// 接口其它方法不参与本测试，全部置为不可达。
func (s *stubFavQuestionRepo) Create(*model.Question) error                { panic("unused") }
func (s *stubFavQuestionRepo) Update(*model.Question) error                { panic("unused") }
func (s *stubFavQuestionRepo) ReplaceOptions(uint64, []model.Option) error { panic("unused") }
func (s *stubFavQuestionRepo) Delete(uint64) error                         { panic("unused") }
func (s *stubFavQuestionRepo) List(repository.QuestionListReq) ([]model.Question, int64, error) {
	panic("unused")
}
func (s *stubFavQuestionRepo) FindByIDs([]uint64) ([]model.Question, error) { panic("unused") }
func (s *stubFavQuestionRepo) GetRandomQuestions(int8, int, string, uint64) ([]model.Question, error) {
	panic("unused")
}
func (s *stubFavQuestionRepo) GetSeededRandomQuestions(int8, int, string, uint64, uint32) ([]model.Question, error) {
	panic("unused")
}
func (s *stubFavQuestionRepo) GetRandomByTypeYear(int8, int, uint64, int16) ([]model.Question, error) {
	panic("unused")
}
func (s *stubFavQuestionRepo) CountQuestions(int8, string, uint64) (int64, error) { panic("unused") }
func (s *stubFavQuestionRepo) UpdateSortOrders([]uint64) error { panic("unused") }
func (s *stubFavQuestionRepo) GetCategories(uint64) ([]repository.Category, error) {
	panic("unused")
}

type stubFavArticleRepo struct{ byID map[uint64]*model.Article }

func (s *stubFavArticleRepo) FindByID(id uint64) (*model.Article, error) {
	if a, ok := s.byID[id]; ok {
		return a, nil
	}
	return nil, errNotFound
}
func (s *stubFavArticleRepo) Create(*model.Article) error { panic("unused") }
func (s *stubFavArticleRepo) Update(*model.Article) error { panic("unused") }
func (s *stubFavArticleRepo) Delete(uint64) error         { panic("unused") }
func (s *stubFavArticleRepo) List(repository.ArticleListReq) ([]model.Article, int64, error) {
	panic("unused")
}

type stubFavKPRepo struct{ byID map[uint64]*model.KnowledgePoint }

func (s *stubFavKPRepo) FindByID(id uint64) (*model.KnowledgePoint, error) {
	if kp, ok := s.byID[id]; ok {
		return kp, nil
	}
	return nil, errNotFound
}
func (s *stubFavKPRepo) Create(*model.KnowledgePoint) error { panic("unused") }
func (s *stubFavKPRepo) Update(*model.KnowledgePoint) error { panic("unused") }
func (s *stubFavKPRepo) Delete(uint64) error               { panic("unused") }
func (s *stubFavKPRepo) List(uint64, uint64, int, int) ([]model.KnowledgePoint, int64, error) {
	panic("unused")
}
func (s *stubFavKPRepo) FindRelatedQuestions([]string, uint64, int) ([]model.RelatedQuestion, error) {
	panic("unused")
}
func (s *stubFavKPRepo) Reorder([]repository.ReorderItem, string) (string, error) {
	panic("unused")
}

func newFavListSvc(f *stubFavRepo, q *stubFavQuestionRepo, a *stubFavArticleRepo, kp *stubFavKPRepo) *FavoriteService {
	return NewFavoriteService(f, q, a, kp)
}

// List 分页后只解析「当前页」的收藏实体：库里有 3 条但 page_size=2，
// 逐条解析必须只发生 2 次（qRepo.calls==2）；第 3 条(第2页)不该被解析。
// total 透传收藏总数（不是当前页条数）。
func TestFavoriteList_ResolvesOnlyCurrentPage_ReturnsTotal(t *testing.T) {
	favRepo := &stubFavRepo{
		favs: []model.Favorite{
			{ID: 1, UserID: 1, TargetType: 1, TargetID: 11},
			{ID: 2, UserID: 1, TargetType: 1, TargetID: 12},
			{ID: 3, UserID: 1, TargetType: 1, TargetID: 13},
		},
		total: 3,
	}
	qRepo := &stubFavQuestionRepo{byID: map[uint64]*model.Question{
		11: {ID: 11, Title: "Q1"},
		12: {ID: 12, Title: "Q2"},
		// 13 故意缺失：实体不存在时应跳过而非让整个列表报错
	}}
	svc := newFavListSvc(favRepo, qRepo, &stubFavArticleRepo{}, &stubFavKPRepo{})

	items, total, err := svc.List(1, 1, 1, 2)
	if err != nil {
		t.Fatalf("List 应无错返回，得到 %v", err)
	}
	if total != 3 {
		t.Fatalf("total 应透传 3，实际 %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("page_size=2 应解析 2 条，实际 %d", len(items))
	}
	if qRepo.calls != 2 {
		t.Fatalf("分页后应只解析当前页 2 条，实际解析 %d 次", qRepo.calls)
	}
	if favRepo.gotPage != 1 || favRepo.gotPageSz != 2 || favRepo.gotTarget != 1 {
		t.Fatalf("分页参数未透传: page=%d size=%d target=%d", favRepo.gotPage, favRepo.gotPageSz, favRepo.gotTarget)
	}
	// 题目 tab 用 Title
	if items[0].ID != 11 || items[0].Title != "Q1" || items[0].Name != "" {
		t.Fatalf("题目收藏应带 ID+Title: %+v", items[0])
	}
}

// 收藏指向的实体可能已被删除（孤儿收藏）——当前页整页都是孤儿时，
// 前端会看到「total>0 却空页」。服务层必须向后翻页回补，凑满当前页。
// 本页 11/12 都是孤儿 → 回补第 2 页把 13/14 顶上来。
func TestFavoriteList_BackfillsWhenCurrentPageAllOrphan(t *testing.T) {
	favRepo := &stubFavRepo{
		favs: []model.Favorite{
			{ID: 1, UserID: 1, TargetType: 1, TargetID: 11}, // 孤儿：题目已删
			{ID: 2, UserID: 1, TargetType: 1, TargetID: 12}, // 孤儿
			{ID: 3, UserID: 1, TargetType: 1, TargetID: 13},
			{ID: 4, UserID: 1, TargetType: 1, TargetID: 14},
		},
		total: 4,
	}
	qRepo := &stubFavQuestionRepo{byID: map[uint64]*model.Question{
		13: {ID: 13, Title: "Q3"},
		14: {ID: 14, Title: "Q4"},
	}}
	svc := newFavListSvc(favRepo, qRepo, &stubFavArticleRepo{}, &stubFavKPRepo{})

	items, total, err := svc.List(1, 1, 1, 2)
	if err != nil {
		t.Fatalf("List 应无错返回，得到 %v", err)
	}
	if total != 4 {
		t.Fatalf("total 应透传收藏总数 4，实际 %d", total)
	}
	// page1 两条都是孤儿 → 回补第 2 页，把存在的 13/14 凑满当前页
	if len(items) != 2 || items[0].ID != 13 || items[1].ID != 14 {
		t.Fatalf("孤儿页应回补为实体存在的收藏: %+v", items)
	}
	if favRepo.calls != 2 {
		t.Fatalf("应翻页到第 2 页回补，实际查询 %d 次", favRepo.calls)
	}
}

// 全库收藏都是孤儿（实体全删）→ 回补翻到越界后必须停下，
// 返回能凑到的（这里是空）+ total，不能死循环。
func TestFavoriteList_BackfillExhaustionStopsAtEnd(t *testing.T) {
	favRepo := &stubFavRepo{
		favs: []model.Favorite{
			{ID: 1, UserID: 1, TargetType: 1, TargetID: 11}, // 孤儿
			{ID: 2, UserID: 1, TargetType: 1, TargetID: 12}, // 孤儿
		},
		total: 2,
	}
	qRepo := &stubFavQuestionRepo{byID: map[uint64]*model.Question{}}
	svc := newFavListSvc(favRepo, qRepo, &stubFavArticleRepo{}, &stubFavKPRepo{})

	items, total, err := svc.List(1, 1, 1, 5)
	if err != nil {
		t.Fatalf("List 应无错返回，得到 %v", err)
	}
	if total != 2 {
		t.Fatalf("total 应透传收藏总数 2，实际 %d", total)
	}
	if len(items) != 0 {
		t.Fatalf("全孤儿应返回空列表，实际 %+v", items)
	}
	// 越界页返回空行即停：首次查询 + 越界探测至多 2 次
	if favRepo.calls > 2 {
		t.Fatalf("越界后仍继续翻页（死循环风险），实际查询 %d 次", favRepo.calls)
	}
}

// 三种 tab 的字段形态：题目/论文用 Title，知识点用 Name。
func TestFavoriteList_EntityShapesPerTab(t *testing.T) {
	favRepo := &stubFavRepo{
		favs: []model.Favorite{
			{ID: 1, UserID: 1, TargetType: 2, TargetID: 21},
			{ID: 2, UserID: 1, TargetType: 3, TargetID: 31},
		},
		total: 2,
	}
	articleRepo := &stubFavArticleRepo{byID: map[uint64]*model.Article{
		21: {ID: 21, Title: "论文A"},
	}}
	kpRepo := &stubFavKPRepo{byID: map[uint64]*model.KnowledgePoint{
		31: {ID: 31, Name: "知识点B"},
	}}
	svc := newFavListSvc(favRepo, &stubFavQuestionRepo{}, articleRepo, kpRepo)

	// 论文 tab
	items, _, err := svc.List(1, 2, 1, 20)
	if err != nil {
		t.Fatalf("论文 tab List 应无错，得到 %v", err)
	}
	if len(items) != 1 || items[0].ID != 21 || items[0].Title != "论文A" || items[0].Name != "" {
		t.Fatalf("论文收藏应带 ID+Title: %+v", items)
	}

	// 知识点 tab
	items, _, err = svc.List(1, 3, 1, 20)
	if err != nil {
		t.Fatalf("知识点 tab List 应无错，得到 %v", err)
	}
	if len(items) != 1 || items[0].ID != 31 || items[0].Name != "知识点B" || items[0].Title != "" {
		t.Fatalf("知识点收藏应带 ID+Name: %+v", items)
	}
}