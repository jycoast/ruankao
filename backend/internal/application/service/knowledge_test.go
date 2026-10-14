package service

import (
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type stubKPRepo struct {
	gotItems       []repository.ReorderItem
	gotBaseVersion string
	returnVersion  string
	returnErr      error
}

func (s *stubKPRepo) Create(*model.KnowledgePoint) error  { panic("unused") }
func (s *stubKPRepo) FindByID(uint64) (*model.KnowledgePoint, error) {
	panic("unused")
}
func (s *stubKPRepo) Update(*model.KnowledgePoint) error { panic("unused") }
func (s *stubKPRepo) Delete(uint64) error               { panic("unused") }
func (s *stubKPRepo) List(uint64, uint64, int, int) ([]model.KnowledgePoint, int64, error) {
	panic("unused")
}
func (s *stubKPRepo) FindRelatedQuestions([]string, uint64, int) ([]model.RelatedQuestion, error) {
	panic("unused")
}
func (s *stubKPRepo) Reorder(items []repository.ReorderItem, baseVersion string) (string, error) {
	s.gotItems = items
	s.gotBaseVersion = baseVersion
	return s.returnVersion, s.returnErr
}

// KnowledgeService.Reorder 是单行 delegate(调 repo.Reorder),锁住这个透传契约:
// 防止后续重构误把业务校验塞进 service 层(校验应该留在 repo 的事务边界,
// 才能被同一事务的 SELECT FOR UPDATE 锁保护)。
func TestKnowledgeService_Reorder_DelegatesToRepo(t *testing.T) {
	stub := &stubKPRepo{returnVersion: "2026-09-05T12:00:00Z"}
	svc := NewKnowledgeService(stub)

	items := []repository.ReorderItem{
		{ID: 1, ParentID: 2, SortOrder: 5},
	}
	version, err := svc.Reorder(items, "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatalf("应透传无错,得到 err=%v", err)
	}
	if version != "2026-09-05T12:00:00Z" {
		t.Fatalf("应透传版本号,得到 %q", version)
	}
	if len(stub.gotItems) != 1 || stub.gotItems[0].ID != 1 || stub.gotItems[0].SortOrder != 5 {
		t.Fatalf("items 应原样透传,得到 %+v", stub.gotItems)
	}
	if stub.gotBaseVersion != "2026-09-01T00:00:00Z" {
		t.Fatalf("baseVersion 应原样透传,得到 %q", stub.gotBaseVersion)
	}
}
