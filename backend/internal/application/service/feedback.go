package service

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type FeedbackService struct {
	repo         repository.FeedbackRepository
	questionRepo repository.QuestionRepository
	notifSvc     *NotificationService
}

func NewFeedbackService(repo repository.FeedbackRepository, questionRepo repository.QuestionRepository, notifSvc *NotificationService) *FeedbackService {
	return &FeedbackService{repo: repo, questionRepo: questionRepo, notifSvc: notifSvc}
}

func (s *FeedbackService) Submit(userID uint64, feedbackType int8, questionID uint64, content, contact string) error {
	// 内容纠错填了题目编号必须真实存在，避免管理员看到死链
	if questionID > 0 {
		if _, err := s.questionRepo.FindByID(questionID); err != nil {
			return errors.New("关联的题目不存在")
		}
	}
	// 富文本兜底 sanitize + 可见字数校验。前端 RichTextEditor 已 sanitize 一道，
	// 此处再保底（与 question_note.Save 的双保险链路一致）：剥 on* 事件/危险协议、
	// <img> src 收窄到 https:/data:image/。返回空串 = 清洗后可见文字不足 5 个字。
	content = sanitizeFeedbackContent(content)
	if content == "" {
		return errors.New("反馈内容至少 5 个字")
	}
	f := &model.Feedback{
		UserID:       userID,
		FeedbackType: feedbackType,
		QuestionID:   questionID,
		Content:      content,
		Contact:      contact,
		Status:       0,
	}
	return s.repo.Create(f)
}

// htmlTagRe 剥标签取可见文字（反馈字数校验/空判据用）。
// 富文本反馈里图片只留 <img src="https://…">，剥掉标签后图片不贡献字数。
var htmlTagRe = regexp.MustCompile(`(?s)<[^>]*>`)

// stripHTMLText 剥标签并把 &nbsp; 归一为空格，返回剩余可见文本。
func stripHTMLText(in string) string {
	s := htmlTagRe.ReplaceAllString(in, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.TrimSpace(s)
}

// sanitizeFeedbackContent 反馈富文本 sanitize + 可见字数校验。
// "至少 5 个字"按可见文字统计（rune 计数，中文 1 字=1 rune），空白与标签、`&nbsp;`
// 都不算数，避免用 HTML 串长度或空格凑数——<p>abcd</p> 的 HTML 有 11 字符却只有 4 个字。
// 不足 5 字返回 ""，Submit 据此拒绝提交。
func sanitizeFeedbackContent(in string) string {
	out := sanitizeNoteHTML(in)
	plain := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1 // 丢弃所有空白，只数可见字符
		}
		return r
	}, stripHTMLText(out))
	if utf8.RuneCountInString(plain) < 5 {
		return ""
	}
	return out
}

func (s *FeedbackService) ListAll(req repository.FeedbackListReq) ([]repository.FeedbackView, int64, error) {
	return s.repo.ListAll(req)
}

func (s *FeedbackService) MyFeedbacks(userID uint64, page, pageSize int) ([]repository.FeedbackView, int64, error) {
	return s.repo.ListByUserID(userID, page, pageSize)
}

func (s *FeedbackService) UpdateStatus(id uint64, status int8, adminNote string) error {
	// 同 QuestionReportService.UpdateStatus：取 user_id 发通知，状态未变则不重复通知
	before, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(id, status, adminNote); err != nil {
		return err
	}
	if before.Status != status && status != 0 {
		title, content := feedbackNotifyText(status, adminNote)
		s.notifSvc.Notify(before.UserID, title, content, BizFeedback, id, "/my/feedbacks")
	}
	return nil
}

func feedbackNotifyText(status int8, adminNote string) (string, string) {
	title := "你的反馈已处理"
	content := "感谢你的建议，我们已经处理。"
	if status == 2 {
		title = "你的反馈已收到"
		content = "感谢你的反馈，我们已收到，暂时不做调整。"
	}
	if adminNote != "" {
		content += "\n\n管理员回复：" + adminNote
	}
	return title, content
}

func (s *FeedbackService) CountPending() (int64, error) {
	return s.repo.CountPending()
}
