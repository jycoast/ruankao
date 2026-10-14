package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// fakeNotifRepo 用内存切片模拟 notifications 表，MarkRead 复刻真实实现里
// 「id + user_id 同时匹配」的条件，用来验证越权防护。
type fakeNotifRepo struct {
	rows      []*model.Notification
	nextID    uint64
	createErr error
}

func (f *fakeNotifRepo) Create(n *model.Notification) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	n.ID = f.nextID
	f.rows = append(f.rows, n)
	return nil
}

func (f *fakeNotifRepo) ListByUser(userID uint64, _, _ int, onlyUnread bool) ([]model.Notification, int64, error) {
	var out []model.Notification
	for _, n := range f.rows {
		if n.UserID != userID || (onlyUnread && n.ReadAt != nil) {
			continue
		}
		out = append(out, *n)
	}
	return out, int64(len(out)), nil
}

func (f *fakeNotifRepo) CountUnread(userID uint64) (int64, error) {
	var c int64
	for _, n := range f.rows {
		if n.UserID == userID && n.ReadAt == nil {
			c++
		}
	}
	return c, nil
}

func (f *fakeNotifRepo) MarkRead(id, userID uint64) error {
	now := time.Now()
	for _, n := range f.rows {
		if n.ID == id && n.UserID == userID && n.ReadAt == nil {
			n.ReadAt = &now
		}
	}
	return nil
}

func (f *fakeNotifRepo) MarkAllRead(userID uint64) error {
	now := time.Now()
	for _, n := range f.rows {
		if n.UserID == userID && n.ReadAt == nil {
			n.ReadAt = &now
		}
	}
	return nil
}

// fakeReportRepo 只实现发通知路径用到的 FindByID / UpdateStatus。
type fakeReportRepo struct {
	row     model.QuestionReport
	updates int
}

func (f *fakeReportRepo) Create(*model.QuestionReport) error { return nil }
func (f *fakeReportRepo) FindByID(uint64) (*model.QuestionReport, error) {
	r := f.row
	return &r, nil
}
func (f *fakeReportRepo) ListAll(int, int, int) ([]repository.QuestionReportView, int64, error) {
	return nil, 0, nil
}
func (f *fakeReportRepo) ListByUserID(uint64, int, int) ([]repository.QuestionReportView, int64, error) {
	return nil, 0, nil
}
func (f *fakeReportRepo) UpdateStatus(_ uint64, status int8, _ string) error {
	f.updates++
	f.row.Status = status
	return nil
}
func (f *fakeReportRepo) CountPending() (int64, error) { return 0, nil }

// 管理员改完备注再点一次保存，状态没变就不该再发一条 —— 用户不该被同一个结果轰炸。
func TestUpdateStatusNotifiesOnlyOnTransition(t *testing.T) {
	notifRepo := &fakeNotifRepo{}
	reportRepo := &fakeReportRepo{row: model.QuestionReport{ID: 5, UserID: 42, Status: 0}}
	svc := NewQuestionReportService(reportRepo, NewNotificationService(notifRepo))

	if err := svc.UpdateStatus(5, 1, "已修正 B 选项"); err != nil {
		t.Fatalf("first update: %v", err)
	}
	if len(notifRepo.rows) != 1 {
		t.Fatalf("首次状态变更应发 1 条通知，实际 %d", len(notifRepo.rows))
	}
	n := notifRepo.rows[0]
	if n.UserID != 42 || n.BizType != BizReport || n.BizID != 5 || n.Link != "/my/reports" {
		t.Errorf("通知内容不对: %+v", n)
	}
	if !strings.Contains(n.Content, "已修正 B 选项") {
		t.Errorf("管理员回复应出现在通知正文里: %q", n.Content)
	}

	if err := svc.UpdateStatus(5, 1, "补充说明"); err != nil {
		t.Fatalf("second update: %v", err)
	}
	if len(notifRepo.rows) != 1 {
		t.Errorf("状态未变化不应重复发通知，实际 %d 条", len(notifRepo.rows))
	}
	if reportRepo.updates != 2 {
		t.Errorf("备注仍应写库，实际更新 %d 次", reportRepo.updates)
	}
}

// 回到待处理（status=0）不是一个值得通知用户的结果。
func TestUpdateStatusSkipsNotifyForPending(t *testing.T) {
	notifRepo := &fakeNotifRepo{}
	repo := &fakeReportRepo{row: model.QuestionReport{ID: 1, UserID: 9, Status: 1}}
	svc := NewQuestionReportService(repo, NewNotificationService(notifRepo))

	if err := svc.UpdateStatus(1, 0, ""); err != nil {
		t.Fatal(err)
	}
	if len(notifRepo.rows) != 0 {
		t.Errorf("改回待处理不应发通知，实际 %d 条", len(notifRepo.rows))
	}
}

// 通知是旁路：写失败只能打日志，不能把管理员的「已处理」操作也一起失败掉。
func TestUpdateStatusSucceedsWhenNotifyFails(t *testing.T) {
	notifRepo := &fakeNotifRepo{createErr: errors.New("notifications 表挂了")}
	repo := &fakeReportRepo{row: model.QuestionReport{ID: 2, UserID: 3, Status: 0}}
	svc := NewQuestionReportService(repo, NewNotificationService(notifRepo))

	if err := svc.UpdateStatus(2, 1, ""); err != nil {
		t.Fatalf("通知失败不应让 UpdateStatus 报错: %v", err)
	}
	if repo.row.Status != 1 {
		t.Error("状态应已落库")
	}
}

// notifSvc 为 nil（例如某处装配漏注入）时不能 panic。
func TestNotifyNilSafe(t *testing.T) {
	var s *NotificationService
	s.Notify(1, "t", "c", BizFeedback, 1, "/x")
	NewNotificationService(nil).Notify(1, "t", "c", BizFeedback, 1, "/x")
	NewNotificationService(&fakeNotifRepo{}).Notify(0, "t", "c", BizFeedback, 1, "/x")
}

// MarkRead 的 user_id 条件不是冗余：少了它任何人都能标记别人的通知已读。
func TestMarkReadRejectsOtherUsers(t *testing.T) {
	repo := &fakeNotifRepo{}
	svc := NewNotificationService(repo)
	svc.Notify(100, "标题", "正文", BizReport, 1, "/my/reports")

	if err := svc.MarkRead(1, 999); err != nil {
		t.Fatal(err)
	}
	if repo.rows[0].ReadAt != nil {
		t.Error("他人 userID 不应能标记已读")
	}
	if c, _ := svc.UnreadCount(100); c != 1 {
		t.Errorf("未读数应仍为 1，实际 %d", c)
	}

	if err := svc.MarkRead(1, 100); err != nil {
		t.Fatal(err)
	}
	if repo.rows[0].ReadAt == nil {
		t.Error("本人应能标记已读")
	}
	if c, _ := svc.UnreadCount(100); c != 0 {
		t.Errorf("标记后未读数应为 0，实际 %d", c)
	}
}

// MarkAllRead 只能清自己的未读，不能顺手把别人的也清了。
func TestMarkAllReadScopedToUser(t *testing.T) {
	repo := &fakeNotifRepo{}
	svc := NewNotificationService(repo)
	svc.Notify(1, "a", "", BizReport, 1, "")
	svc.Notify(2, "b", "", BizReport, 2, "")

	if err := svc.MarkAllRead(1); err != nil {
		t.Fatal(err)
	}
	if c, _ := svc.UnreadCount(2); c != 1 {
		t.Errorf("其他用户的未读不应被清掉，实际剩 %d", c)
	}
}
