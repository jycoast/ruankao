package service

import (
	"testing"

	"software-exam/internal/domain/model"
)

// GetWrongQuestions 是分页 delegate：把 page/pageSize/status 原样交给 repo，
// total 从 repo 透传。锁住这个契约，防止重构时把分页参数在 service 层吞掉。
func TestGetWrongQuestions_PassesPagination(t *testing.T) {
	svc, _, wr, _ := newSubmitSvc(&model.Question{ID: 1})
	wr.wqs = []model.WrongQuestion{{ID: 1, UserID: 1, QuestionID: 1}}
	wr.total = 7

	list, total, err := svc.GetWrongQuestions(1, 1, 3, 10)
	if err != nil {
		t.Fatalf("GetWrongQuestions 应无错，得到 %v", err)
	}
	if total != 7 || len(list) != 1 {
		t.Fatalf("应透传 total=7 / 1 条，实际 total=%d len=%d", total, len(list))
	}
	if wr.gotStatus != 1 || wr.gotPage != 3 || wr.gotPageSize != 10 {
		t.Fatalf("分页参数未透传给 repo: status=%d page=%d size=%d",
			wr.gotStatus, wr.gotPage, wr.gotPageSize)
	}
}

func TestGetDueReview_PassesPagination(t *testing.T) {
	svc, _, wr, _ := newSubmitSvc(&model.Question{ID: 1})
	wr.wqs = []model.WrongQuestion{{ID: 2, UserID: 1, QuestionID: 2}}
	wr.total = 3

	list, total, err := svc.GetDueReview(1, 2, 5)
	if err != nil {
		t.Fatalf("GetDueReview 应无错，得到 %v", err)
	}
	if total != 3 || len(list) != 1 {
		t.Fatalf("应透传 total=3 / 1 条，实际 total=%d len=%d", total, len(list))
	}
	if wr.gotPage != 2 || wr.gotPageSize != 5 {
		t.Fatalf("分页参数未透传给 repo: page=%d size=%d", wr.gotPage, wr.gotPageSize)
	}
}