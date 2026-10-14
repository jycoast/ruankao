package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"software-exam/internal/domain/model"
)

// ── fake ──

type fakeCommentRepo struct {
	rows         []*model.QuestionComment
	nextID       uint64
	notFound     error
	setStatusErr error
}

func (f *fakeCommentRepo) Create(c *model.QuestionComment) error {
	f.nextID++
	c.ID = f.nextID
	c.CreatedAt = time.Now()
	f.rows = append(f.rows, c)
	return nil
}

func (f *fakeCommentRepo) FindByID(id uint64) (*model.QuestionComment, error) {
	for _, c := range f.rows {
		if c.ID == id {
			cp := *c
			return &cp, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeCommentRepo) ListTop(questionID uint64, _, _ int) ([]model.CommentWithUser, int64, error) {
	var out []model.CommentWithUser
	var total int64
	for _, c := range f.rows {
		if c.QuestionID == questionID && c.ParentID == 0 && c.Status == 0 {
			total++
			out = append(out, model.CommentWithUser{QuestionComment: *c, Nickname: "u"})
		}
	}
	return out, total, nil
}

func (f *fakeCommentRepo) ListReplies(questionID uint64, parentIDs []uint64) ([]model.CommentWithUser, error) {
	var out []model.CommentWithUser
	for _, c := range f.rows {
		if c.QuestionID == questionID && contains(parentIDs, c.ParentID) && c.Status == 0 {
			out = append(out, model.CommentWithUser{QuestionComment: *c, Nickname: "r"})
		}
	}
	return out, nil
}

func (f *fakeCommentRepo) CountReplies(parentIDs []uint64) (map[uint64]int64, error) {
	m := map[uint64]int64{}
	for _, c := range f.rows {
		if c.Status == 0 && contains(parentIDs, c.ParentID) {
			m[c.ParentID]++
		}
	}
	return m, nil
}

func (f *fakeCommentRepo) SoftDelete(id, userID uint64) (int64, error) {
	for _, c := range f.rows {
		if c.ID == id && c.UserID == userID && c.Status == 0 {
			c.Status = 1
			return 1, nil
		}
	}
	return 0, nil
}

func (f *fakeCommentRepo) SetStatus(id uint64, status int8) error {
	if f.setStatusErr != nil {
		return f.setStatusErr
	}
	for _, c := range f.rows {
		if c.ID == id {
			c.Status = status
		}
	}
	return nil
}

func (f *fakeCommentRepo) AdminList(status int, keyword string, _ uint64, _, _ int) ([]model.AdminCommentView, int64, error) {
	var out []model.AdminCommentView
	var total int64
	for _, c := range f.rows {
		if c.Status == 1 {
			continue
		}
		if status > 0 && int(c.Status) != status {
			continue
		}
		if keyword != "" && !strings.Contains(c.Content, keyword) {
			continue
		}
		total++
		out = append(out, model.AdminCommentView{QuestionComment: *c, Nickname: "u"})
	}
	return out, total, nil
}

type fakeCommentReportRepo struct {
	rows   []*model.CommentReport
	nextID uint64
}

func (f *fakeCommentReportRepo) Create(r *model.CommentReport) error {
	f.nextID++
	r.ID = f.nextID
	r.CreatedAt = time.Now()
	f.rows = append(f.rows, r)
	return nil
}

func (f *fakeCommentReportRepo) FindByID(id uint64) (*model.CommentReport, error) {
	for _, r := range f.rows {
		if r.ID == id {
			cp := *r
			return &cp, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeCommentReportRepo) CountByUserAndComment(userID, commentID uint64) (int64, error) {
	var n int64
	for _, r := range f.rows {
		if r.UserID == userID && r.CommentID == commentID {
			n++
		}
	}
	return n, nil
}

func (f *fakeCommentReportRepo) List(status int, _, _ int) ([]model.AdminCommentReportView, int64, error) {
	var out []model.AdminCommentReportView
	var total int64
	for _, r := range f.rows {
		if status >= 0 && int(r.Status) != status {
			continue
		}
		total++
		out = append(out, model.AdminCommentReportView{CommentReport: *r})
	}
	return out, total, nil
}

func (f *fakeCommentReportRepo) SetStatus(id uint64, status int8, adminNote string) error {
	for _, r := range f.rows {
		if r.ID == id {
			r.Status = status
			r.AdminNote = adminNote
		}
	}
	return nil
}

func contains(ids []uint64, id uint64) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

// ── 用例 ──

func newCommentService() (*CommentService, *fakeCommentRepo, *fakeNotifRepo) {
	cr := &fakeCommentRepo{}
	rr := &fakeCommentReportRepo{}
	nr := &fakeNotifRepo{}
	// 沿用 notification_test.go 的写法：fakeNotifRepo 是仓库，需用 NewNotificationService 包一层
	return NewCommentService(cr, rr, NewNotificationService(nr)), cr, nr
}

func TestCommentCreateValidation(t *testing.T) {
	svc, _, _ := newCommentService()
	if err := svc.Create(1, 1, "   ", 0); err != ErrCommentEmpty {
		t.Fatalf("empty content: got %v, want ErrCommentEmpty", err)
	}
	if err := svc.Create(1, 1, strings.Repeat("a", CommentMaxLen+1), 0); err != ErrCommentTooLong {
		t.Fatalf("too long: got %v, want ErrCommentTooLong", err)
	}
	if err := svc.Create(1, 1, "ok", 0); err != nil {
		t.Fatalf("valid create failed: %v", err)
	}
}

func TestCommentReplyValidationsAndNotify(t *testing.T) {
	svc, cr, nr := newCommentService()
	if err := svc.Create(1, 1, "top", 0); err != nil {
		t.Fatal(err)
	}
	parent := cr.rows[0]

	// 回复不存在的父评论
	if err := svc.Create(1, 2, "x", 999); err != ErrCommentNotFound {
		t.Fatalf("reply to missing: got %v, want ErrCommentNotFound", err)
	}
	// 正常回复 → 通知父作者(user 1),且是 user 2 发的
	if err := svc.Create(1, 2, "reply", parent.ID); err != nil {
		t.Fatalf("reply failed: %v", err)
	}
	if len(nr.rows) != 1 || nr.rows[0].UserID != 1 {
		t.Fatalf("reply notify not sent to parent author: %+v", nr.rows)
	}
	// 自己回复自己 → 不通知
	if err := svc.Create(1, 1, "self reply", parent.ID); err != nil {
		t.Fatal(err)
	}
	if len(nr.rows) != 1 {
		t.Fatalf("self-reply should not notify, got %d notifications", len(nr.rows))
	}
	// 回复一个回复(楼中楼套楼中楼)→ 拒绝
	rep := cr.rows[1]
	if err := svc.Create(1, 3, "nested", rep.ID); err != ErrCommentNotFound {
		t.Fatalf("reply to reply: got %v, want ErrCommentNotFound", err)
	}
}

func TestCommentDeleteForbidden(t *testing.T) {
	svc, cr, _ := newCommentService()
	_ = svc.Create(1, 1, "top", 0)
	id := cr.rows[0].ID
	if err := svc.Delete(id, 2); err != ErrCommentForbidden {
		t.Fatalf("delete others: got %v, want ErrCommentForbidden", err)
	}
	if err := svc.Delete(id, 1); err != nil {
		t.Fatalf("delete own failed: %v", err)
	}
	if cr.rows[0].Status != 1 {
		t.Fatalf("soft delete: status=%d, want 1", cr.rows[0].Status)
	}
}

func TestCommentReportDuplicate(t *testing.T) {
	svc, cr, _ := newCommentService()
	_ = svc.Create(1, 1, "top", 0)
	id := cr.rows[0].ID
	if err := svc.Report(id, 2, "垃圾广告"); err != nil {
		t.Fatalf("first report failed: %v", err)
	}
	if err := svc.Report(id, 2, "垃圾广告"); err != ErrCommentReported {
		t.Fatalf("duplicate report: got %v, want ErrCommentReported", err)
	}
	if err := svc.Report(id, 3, "人身攻击"); err != nil {
		t.Fatalf("report by another user failed: %v", err)
	}
}

func TestCommentListAssemble(t *testing.T) {
	svc, cr, _ := newCommentService()
	_ = svc.Create(1, 1, "top1", 0)
	_ = svc.Create(1, 2, "top2", 0)
	p1 := cr.rows[0].ID
	_ = svc.Create(1, 3, "r1", p1)
	_ = svc.Create(1, 4, "r2", p1)

	lst, err := svc.List(1, 1, 20, 3)
	if err != nil {
		t.Fatal(err)
	}
	if lst.Total != 2 {
		t.Fatalf("total=%d, want 2", lst.Total)
	}
	// top1 有 2 条回复,top2 0 条
	if lst.Items[0].ReplyCount != 2 {
		t.Fatalf("top1 reply_count=%d, want 2", lst.Items[0].ReplyCount)
	}
	if lst.Items[1].ReplyCount != 0 {
		t.Fatalf("top2 reply_count=%d, want 0", lst.Items[1].ReplyCount)
	}
	if len(lst.Items[1].Replies) != 0 {
		t.Fatalf("top2 replies should be empty")
	}
	// 删除 top2(soft)后列表只剩 top1
	_ = svc.Delete(lst.Items[1].ID, 2)
	lst2, _ := svc.List(1, 1, 20, 0)
	if lst2.Total != 1 {
		t.Fatalf("after delete total=%d, want 1", lst2.Total)
	}
}

func TestCommentReplyParentDifferentQuestion(t *testing.T) {
	svc, cr, nr := newCommentService()
	// 问题 2 上有一条顶级评论
	if err := svc.Create(2, 1, "top on question2", 0); err != nil {
		t.Fatal(err)
	}
	parent := cr.rows[0]
	before := len(cr.rows)

	// 在问题 1 上回复问题 2 的顶级评论 → 拒绝(防孤儿行)
	if err := svc.Create(1, 2, "reply", parent.ID); err != ErrCommentNotFound {
		t.Fatalf("cross-question reply: got %v, want ErrCommentNotFound", err)
	}
	// 未落库、未通知
	if len(cr.rows) != before {
		t.Fatalf("cross-question reply should not be stored")
	}
	if len(nr.rows) != 0 {
		t.Fatalf("cross-question reply should not notify, got %d notifications", len(nr.rows))
	}
}

func TestCommentHandleReportDelete(t *testing.T) {
	cr := &fakeCommentRepo{}
	rr := &fakeCommentReportRepo{}
	nr := &fakeNotifRepo{}
	svc := NewCommentService(cr, rr, NewNotificationService(nr))

	if err := svc.Create(1, 1, "bad comment", 0); err != nil {
		t.Fatal(err)
	}
	cid := cr.rows[0].ID
	if err := svc.Report(cid, 2, "违规"); err != nil {
		t.Fatal(err)
	}
	rid := rr.rows[0].ID

	if err := svc.HandleReport(rid, "delete", "确认违规"); err != nil {
		t.Fatalf("HandleReport failed: %v", err)
	}
	// 评论被隐藏、举报被处理
	if cr.rows[0].Status != 2 {
		t.Fatalf("comment status=%d, want 2", cr.rows[0].Status)
	}
	if rr.rows[0].Status != 1 {
		t.Fatalf("report status=%d, want 1", rr.rows[0].Status)
	}
	// 通知举报人 + 作者
	if len(nr.rows) != 2 {
		t.Fatalf("notifications=%d, want 2 (reporter + author)", len(nr.rows))
	}
	// 幂等:已处理的举报再次 delete → nil,不再发通知
	if err := svc.HandleReport(rid, "delete", "again"); err != nil {
		t.Fatalf("idempotent HandleReport failed: %v", err)
	}
	if len(nr.rows) != 2 {
		t.Fatalf("idempotent re-handle should not notify, got %d notifications", len(nr.rows))
	}
}

func TestCommentHandleReportHideFailure(t *testing.T) {
	cr := &fakeCommentRepo{}
	rr := &fakeCommentReportRepo{}
	nr := &fakeNotifRepo{}
	svc := NewCommentService(cr, rr, NewNotificationService(nr))

	if err := svc.Create(1, 1, "bad comment", 0); err != nil {
		t.Fatal(err)
	}
	cid := cr.rows[0].ID
	if err := svc.Report(cid, 2, "违规"); err != nil {
		t.Fatal(err)
	}
	rid := rr.rows[0].ID

	cr.setStatusErr = errors.New("db down")

	if err := svc.HandleReport(rid, "delete", "确认违规"); err == nil {
		t.Fatalf("HandleReport should propagate SetStatus error")
	}
	// 举报未被处理、评论未被隐藏、未发任何通知
	if rr.rows[0].Status != 0 {
		t.Fatalf("report status=%d, want 0 (unprocessed)", rr.rows[0].Status)
	}
	if cr.rows[0].Status != 0 {
		t.Fatalf("comment status=%d, want 0 (not hidden)", cr.rows[0].Status)
	}
	if len(nr.rows) != 0 {
		t.Fatalf("hide failure should not notify, got %d notifications", len(nr.rows))
	}
}

func TestCommentHandleReportAlreadyHidden(t *testing.T) {
	cr := &fakeCommentRepo{}
	rr := &fakeCommentReportRepo{}
	nr := &fakeNotifRepo{}
	svc := NewCommentService(cr, rr, NewNotificationService(nr))

	if err := svc.Create(1, 1, "already hidden", 0); err != nil {
		t.Fatal(err)
	}
	cid := cr.rows[0].ID
	if err := svc.Report(cid, 2, "违规"); err != nil {
		t.Fatal(err)
	}
	rid := rr.rows[0].ID
	// 预置:评论已在早前处理中被隐藏
	cr.rows[0].Status = 2

	if err := svc.HandleReport(rid, "delete", "确认违规"); err != nil {
		t.Fatalf("HandleReport failed: %v", err)
	}
	// 举报仍被处理
	if rr.rows[0].Status != 1 {
		t.Fatalf("report status=%d, want 1 (processed)", rr.rows[0].Status)
	}
	// 只通知举报人,不再重复通知作者
	if len(nr.rows) != 1 {
		t.Fatalf("notifications=%d, want 1 (reporter only)", len(nr.rows))
	}
	if nr.rows[0].BizType != BizCommentReport {
		t.Fatalf("reporter notification biz_type=%q, want %q", nr.rows[0].BizType, BizCommentReport)
	}
}
