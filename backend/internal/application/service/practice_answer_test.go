package service

import (
	"testing"

	"software-exam/internal/domain/model"
)

func TestBuildCorrectAnswerSingle(t *testing.T) {
	opts := []model.Option{
		{Label: "A", IsCorrect: 0, SortOrder: 0},
		{Label: "B", IsCorrect: 1, SortOrder: 1},
		{Label: "C", IsCorrect: 0, SortOrder: 2},
		{Label: "D", IsCorrect: 0, SortOrder: 3},
	}
	if got := buildCorrectAnswer(opts); got != "B" {
		t.Fatalf("got %q want B", got)
	}
}

func TestBuildCorrectAnswerMultiClean(t *testing.T) {
	opts := []model.Option{
		{ID: 1, Label: "A", IsCorrect: 0, SortOrder: 0},
		{ID: 2, Label: "B", IsCorrect: 0, SortOrder: 1},
		{ID: 3, Label: "C", IsCorrect: 1, SortOrder: 2},
		{ID: 4, Label: "D", IsCorrect: 0, SortOrder: 3},
		{ID: 5, Label: "A", IsCorrect: 0, SortOrder: 4},
		{ID: 6, Label: "B", IsCorrect: 0, SortOrder: 5},
		{ID: 7, Label: "C", IsCorrect: 0, SortOrder: 6},
		{ID: 8, Label: "D", IsCorrect: 1, SortOrder: 7},
	}
	if got := buildCorrectAnswer(opts); got != "CD" {
		t.Fatalf("got %q want CD", got)
	}
}

// 企业应用集成那类导入脏数据：两组都把 C/D 标成正确，且 sort_order 每组 dual 0..3。
func TestBuildCorrectAnswerMultiDirtySharedCorrects(t *testing.T) {
	opts := []model.Option{
		{ID: 1, Label: "A", IsCorrect: 0, SortOrder: 0},
		{ID: 2, Label: "B", IsCorrect: 0, SortOrder: 1},
		{ID: 3, Label: "C", IsCorrect: 1, SortOrder: 2},
		{ID: 4, Label: "D", IsCorrect: 1, SortOrder: 3},
		{ID: 5, Label: "A", IsCorrect: 0, SortOrder: 0},
		{ID: 6, Label: "B", IsCorrect: 0, SortOrder: 1},
		{ID: 7, Label: "C", IsCorrect: 1, SortOrder: 2},
		{ID: 8, Label: "D", IsCorrect: 1, SortOrder: 3},
	}
	if got := buildCorrectAnswer(opts); got != "CD" {
		t.Fatalf("got %q want CD (dirty shared corrects)", got)
	}
}

func TestSubmitMultiBlankDirtyData(t *testing.T) {
	q := &model.Question{ID: 1, QuestionType: 1, Options: []model.Option{
		{ID: 1, Label: "A", IsCorrect: 0, SortOrder: 0},
		{ID: 2, Label: "B", IsCorrect: 0, SortOrder: 1},
		{ID: 3, Label: "C", IsCorrect: 1, SortOrder: 2},
		{ID: 4, Label: "D", IsCorrect: 1, SortOrder: 3},
		{ID: 5, Label: "A", IsCorrect: 0, SortOrder: 0},
		{ID: 6, Label: "B", IsCorrect: 0, SortOrder: 1},
		{ID: 7, Label: "C", IsCorrect: 1, SortOrder: 2},
		{ID: 8, Label: "D", IsCorrect: 1, SortOrder: 3},
	}}
	svc, _, wr, _ := newSubmitSvc(q)
	res, err := svc.SubmitAnswer(1, 1, "CD", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsCorrect {
		t.Fatalf("CD should be correct, answer=%s", res.Answer)
	}
	if res.Answer != "CD" {
		t.Fatalf("answer got %q want CD", res.Answer)
	}
	if wr.created != 0 {
		t.Fatal("should not enter wrong book")
	}
}
