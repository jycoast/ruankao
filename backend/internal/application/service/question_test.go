package service

import (
	"errors"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type fakeQuestionRepo struct {
	q            *model.Question
	options      []model.Option
	replaceCalls int
}

func (f *fakeQuestionRepo) Create(q *model.Question) error {
	q.ID = 1
	f.q = q
	return nil
}

func (f *fakeQuestionRepo) FindByID(id uint64) (*model.Question, error) {
	if f.q == nil || f.q.ID != id {
		return nil, errors.New("not found")
	}
	cp := *f.q
	return &cp, nil
}

func (f *fakeQuestionRepo) Update(q *model.Question) error { f.q = q; return nil }

func (f *fakeQuestionRepo) ReplaceOptions(questionID uint64, opts []model.Option) error {
	f.replaceCalls++
	f.options = opts
	return nil
}

func (f *fakeQuestionRepo) Delete(uint64) error { panic("unused") }
func (f *fakeQuestionRepo) List(repository.QuestionListReq) ([]model.Question, int64, error) {
	panic("unused")
}
func (f *fakeQuestionRepo) FindByIDs([]uint64) ([]model.Question, error) { panic("unused") }
func (f *fakeQuestionRepo) UpdateSortOrders([]uint64) error              { panic("unused") }
func (f *fakeQuestionRepo) GetRandomQuestions(int8, int, string, uint64) ([]model.Question, error) {
	panic("unused")
}
func (f *fakeQuestionRepo) GetSeededRandomQuestions(int8, int, string, uint64, uint32) ([]model.Question, error) {
	panic("unused")
}
func (f *fakeQuestionRepo) GetRandomByTypeYear(int8, int, uint64, int16) ([]model.Question, error) {
	panic("unused")
}
func (f *fakeQuestionRepo) CountQuestions(int8, string, uint64) (int64, error) { panic("unused") }
func (f *fakeQuestionRepo) GetCategories(uint64) ([]repository.Category, error) { panic("unused") }

// 原实现的 Update 完全不碰 options 表：管理员改了正确答案，
// 界面提示"保存成功"但库里纹丝不动，爬来的错答案永远修不掉。
func TestUpdateWritesOptions(t *testing.T) {
	repo := &fakeQuestionRepo{q: &model.Question{ID: 1, Title: "旧题干"}}
	svc := NewQuestionService(repo)

	err := svc.Update(1, &QuestionRequest{
		Title: "新题干",
		Options: []QuestionOptionRequest{
			{Label: "A", Content: "错误项", IsCorrect: 0},
			{Label: "B", Content: "正确项", IsCorrect: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.replaceCalls != 1 {
		t.Fatalf("应写入一次选项，实际 %d 次", repo.replaceCalls)
	}
	if len(repo.options) != 2 || repo.options[1].IsCorrect != 1 {
		t.Fatalf("正确答案未落库: %+v", repo.options)
	}
}

// 只改题干不传 options 时不应把选项清空。
func TestUpdateWithoutOptionsLeavesThemAlone(t *testing.T) {
	repo := &fakeQuestionRepo{q: &model.Question{ID: 1, Title: "旧题干"}}
	svc := NewQuestionService(repo)

	if err := svc.Update(1, &QuestionRequest{Title: "只改题干"}); err != nil {
		t.Fatal(err)
	}
	if repo.replaceCalls != 0 {
		t.Fatal("未传 options 时不应改动选项表")
	}
}

func TestCreateWritesOptions(t *testing.T) {
	repo := &fakeQuestionRepo{}
	svc := NewQuestionService(repo)

	id, err := svc.Create(&QuestionRequest{
		Title:   "新题",
		Options: []QuestionOptionRequest{{Content: "甲"}, {Content: "乙", IsCorrect: 1}},
	})
	if err != nil || id == 0 {
		t.Fatalf("创建失败: id=%d err=%v", id, err)
	}
	if repo.replaceCalls != 1 || len(repo.options) != 2 {
		t.Fatalf("新建题目应写入选项，实际 calls=%d opts=%d", repo.replaceCalls, len(repo.options))
	}
	// 未传 label 时按 A/B/C 补齐
	if repo.options[0].Label != "A" || repo.options[1].Label != "B" {
		t.Fatalf("标号补齐有误: %+v", repo.options)
	}
}
