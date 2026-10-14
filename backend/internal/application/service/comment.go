package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

const CommentMaxLen = 500

var (
	ErrCommentEmpty     = errors.New("评论内容不能为空")
	ErrCommentTooLong   = fmt.Errorf("评论内容不能超过 %d 字", CommentMaxLen)
	ErrCommentNotFound  = errors.New("评论不存在")
	ErrCommentForbidden = errors.New("无权操作该评论")
	ErrCommentReported  = errors.New("你已举报过这条评论")
)

type CommentService struct {
	repo       repository.CommentRepository
	reportRepo repository.CommentReportRepository
	notifSvc   *NotificationService
}

func NewCommentService(repo repository.CommentRepository, reportRepo repository.CommentReportRepository, notifSvc *NotificationService) *CommentService {
	return &CommentService{repo: repo, reportRepo: reportRepo, notifSvc: notifSvc}
}

// ── 返回视图 ──

type CommentUser struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type ReplyView struct {
	ID            uint64      `json:"id"`
	Content       string      `json:"content"`
	User          CommentUser `json:"user"`
	ReplyToUserID uint64      `json:"reply_to_user_id"`
	IsOwner       bool        `json:"is_owner"`
	CreatedAt     time.Time   `json:"created_at"`
}

type CommentView struct {
	ID         uint64      `json:"id"`
	Content    string      `json:"content"`
	User       CommentUser `json:"user"`
	ReplyCount int64       `json:"reply_count"`
	Replies    []ReplyView `json:"replies"`
	IsOwner    bool        `json:"is_owner"`
	CreatedAt  time.Time   `json:"created_at"`
}

type CommentList struct {
	Total int64         `json:"total"`
	Items []CommentView `json:"items"`
}

// ── 用户端 ──

func (s *CommentService) List(questionID uint64, page, pageSize int, currentUser uint64) (*CommentList, error) {
	top, total, err := s.repo.ListTop(questionID, page, pageSize)
	if err != nil {
		return nil, err
	}
	parentIDs := make([]uint64, 0, len(top))
	for _, c := range top {
		parentIDs = append(parentIDs, c.ID)
	}
	replies, err := s.repo.ListReplies(questionID, parentIDs)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.CountReplies(parentIDs)
	if err != nil {
		return nil, err
	}
	byParent := map[uint64][]ReplyView{}
	for _, rp := range replies {
		byParent[rp.ParentID] = append(byParent[rp.ParentID], ReplyView{
			ID:            rp.ID,
			Content:       rp.Content,
			User:          CommentUser{Nickname: rp.Nickname, Avatar: rp.Avatar},
			ReplyToUserID: rp.ReplyToUserID,
			IsOwner:       rp.UserID == currentUser,
			CreatedAt:     rp.CreatedAt,
		})
	}
	items := make([]CommentView, 0, len(top))
	for _, c := range top {
		items = append(items, CommentView{
			ID:         c.ID,
			Content:    c.Content,
			User:       CommentUser{Nickname: c.Nickname, Avatar: c.Avatar},
			ReplyCount: counts[c.ID],
			Replies:    byParent[c.ID],
			IsOwner:    c.UserID == currentUser,
			CreatedAt:  c.CreatedAt,
		})
	}
	return &CommentList{Total: total, Items: items}, nil
}

func (s *CommentService) Create(questionID, userID uint64, content string, parentID uint64) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrCommentEmpty
	}
	if len([]rune(content)) > CommentMaxLen {
		return ErrCommentTooLong
	}
	c := &model.QuestionComment{
		QuestionID: questionID,
		UserID:     userID,
		Content:    content,
		Status:     0,
	}
	if parentID > 0 {
		parent, err := s.repo.FindByID(parentID)
		if err != nil || parent.QuestionID != questionID || parent.ParentID != 0 || parent.Status != 0 {
			return ErrCommentNotFound
		}
		c.ParentID = parentID
		c.ReplyToUserID = parent.UserID
	}
	if err := s.repo.Create(c); err != nil {
		return err
	}
	if c.ReplyToUserID != 0 && c.ReplyToUserID != userID {
		s.notifSvc.Notify(c.ReplyToUserID, "有人回复了你的评论",
			truncate(content, 80),
			BizComment, c.ID, fmt.Sprintf("/questions/%d", questionID))
	}
	return nil
}

func (s *CommentService) Delete(id, userID uint64) error {
	n, err := s.repo.SoftDelete(id, userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrCommentForbidden
	}
	return nil
}

func (s *CommentService) Report(commentID, userID uint64, reason string) error {
	if _, err := s.repo.FindByID(commentID); err != nil {
		return ErrCommentNotFound
	}
	n, err := s.reportRepo.CountByUserAndComment(userID, commentID)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrCommentReported
	}
	return s.reportRepo.Create(&model.CommentReport{
		CommentID: commentID,
		UserID:    userID,
		Reason:    reason,
		Status:    0,
	})
}

// ── 管理端 ──

func (s *CommentService) AdminList(status int, keyword string, questionID uint64, page, pageSize int) ([]model.AdminCommentView, int64, error) {
	return s.repo.AdminList(status, keyword, questionID, page, pageSize)
}

func (s *CommentService) SetStatus(id uint64, status int8) error {
	before, err := s.repo.FindByID(id)
	if err != nil {
		return ErrCommentNotFound
	}
	if err := s.repo.SetStatus(id, status); err != nil {
		return err
	}
	// 隐藏时通知作者;状态未变不重复打扰
	if status == 2 && before.Status != 2 {
		s.notifSvc.Notify(before.UserID, "你的评论因违规被隐藏",
			"你的评论「"+truncate(before.Content, 40)+"」因违反社区规范被管理员隐藏。",
			BizComment, id, "")
	}
	return nil
}

func (s *CommentService) ReportList(status int, page, pageSize int) ([]model.AdminCommentReportView, int64, error) {
	return s.reportRepo.List(status, page, pageSize)
}

func (s *CommentService) HandleReport(id uint64, action string, adminNote string) error {
	rep, err := s.reportRepo.FindByID(id)
	if err != nil {
		return ErrCommentNotFound
	}
	switch action {
	case "delete":
		if rep.Status != 0 {
			return nil // 已处理过,幂等
		}
		// 隐藏被举报评论
		cmt, err := s.repo.FindByID(rep.CommentID)
		if err != nil {
			return err
		}
		if cmt.Status != 2 {
			if err := s.repo.SetStatus(rep.CommentID, 2); err != nil {
				return err
			}
		}
		if err := s.reportRepo.SetStatus(id, 1, adminNote); err != nil {
			return err
		}
		// 通知举报人已处理
		s.notifSvc.Notify(rep.UserID, "你举报的评论已处理",
			"感谢反馈,该评论已被隐藏。", BizCommentReport, id, "")
		// 仅当本次确实执行了隐藏且非作者自举报时通知作者
		if cmt.Status != 2 && cmt.UserID != rep.UserID {
			s.notifSvc.Notify(cmt.UserID, "你的评论因违规被隐藏",
				"你的评论「"+truncate(cmt.Content, 40)+"」因违反社区规范被管理员隐藏。",
				BizComment, rep.CommentID, "")
		}
	case "dismiss":
		if rep.Status != 0 {
			return nil
		}
		return s.reportRepo.SetStatus(id, 2, adminNote)
	default:
		return errors.New("无效的操作类型")
	}
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
