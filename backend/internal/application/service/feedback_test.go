package service

import (
	"strings"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// 反馈富文本 sanitize + 可见字数校验的纯函数测试（沿袭 question_note_test.go 的风格）。

func TestSanitizeFeedbackContent_CountsVisibleChars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // 期望返回值；"" 表示应被拒
	}{
		{"五字通过", "<p>你好软考星图</p>", "<p>你好软考星图</p>"},
		{"四字拒绝", "<p>abcd</p>", ""},
		{"空段落拒绝", "<p></p>", ""},
		{"纯图片不计数", `<p></p><img src="https://x.com/a.png"><p></p>`, ""},
		{"nbsp 凑数拒绝", "<p>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</p>", ""},
		{"标签间空白不计", "<p>你好</p> <p>软考星图</p>", "<p>你好</p> <p>软考星图</p>"},
		{"粗体也算字", "<p><strong>你好软考星图</strong></p>", "<p><strong>你好软考星图</strong></p>"},
	}
	for _, c := range cases {
		if got := sanitizeFeedbackContent(c.in); got != c.want {
			t.Errorf("%s: sanitizeFeedbackContent(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestSanitizeFeedbackContent_StripsEventsAndDangerousSrc(t *testing.T) {
	// 5 个可见字本应通过，但 on* 事件与 javascript: 必须被剥掉
	in := `<p onclick="alert(1)">你好软考星图</p><img src="javascript:alert(1)">`
	out := sanitizeFeedbackContent(in)
	if out == "" {
		t.Fatal("五字内容本应通过，返回空")
	}
	if strings.Contains(out, "onclick") {
		t.Errorf("onclick 未剥离: %s", out)
	}
	if strings.Contains(strings.ToLower(out), "javascript:") {
		t.Errorf("javascript: 未剥离: %s", out)
	}
}

func TestSanitizeFeedbackContent_KeepsHTTPSImg(t *testing.T) {
	in := `<p>这个问题如图</p><img src="https://bucket.cos.ap-shanghai.myqcloud.com/2026/01/01/a.png" alt="截图">`
	out := sanitizeFeedbackContent(in)
	if out == "" {
		t.Fatal("含文字+图片本应通过，返回空")
	}
	if !strings.Contains(out, `src="https://bucket.cos.ap-shanghai.myqcloud.com/2026/01/01/a.png"`) {
		t.Errorf("https 图片 src 应保留: %s", out)
	}
}

func TestSanitizeFeedbackContent_Empty(t *testing.T) {
	if got := sanitizeFeedbackContent(""); got != "" {
		t.Errorf("空串应返回空，got %q", got)
	}
}

// ---- Submit 链路：字数校验挡住 + 入库前 sanitize ----

type stubFeedbackRepo struct {
	created *model.Feedback
}

func (s *stubFeedbackRepo) Create(f *model.Feedback) error {
	s.created = f
	return nil
}
func (s *stubFeedbackRepo) FindByID(uint64) (*model.Feedback, error) { panic("unused") }
func (s *stubFeedbackRepo) ListAll(repository.FeedbackListReq) ([]repository.FeedbackView, int64, error) {
	panic("unused")
}
func (s *stubFeedbackRepo) ListByUserID(uint64, int, int) ([]repository.FeedbackView, int64, error) {
	panic("unused")
}
func (s *stubFeedbackRepo) UpdateStatus(uint64, int8, string) error { panic("unused") }
func (s *stubFeedbackRepo) CountPending() (int64, error)            { panic("unused") }

func TestFeedbackSubmit_RejectsTooShortContent(t *testing.T) {
	repo := &stubFeedbackRepo{}
	// questionID=0，Submit 不该触碰题目表；stubNoteQuestionRepo 的 FindByID 是"不可达即 panic"
	svc := NewFeedbackService(repo, &stubNoteQuestionRepo{}, nil)

	err := svc.Submit(7, 2, 0, `<p>短</p>`, "")
	if err == nil || !strings.Contains(err.Error(), "至少 5 个字") {
		t.Fatalf("过短内容应被拒，got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("被拒内容不该落库，got %+v", repo.created)
	}
}

func TestFeedbackSubmit_SanitizesAndStores(t *testing.T) {
	repo := &stubFeedbackRepo{}
	svc := NewFeedbackService(repo, &stubNoteQuestionRepo{}, nil)

	err := svc.Submit(7, 2, 0, `<p onclick="alert(1)">你好软考星图</p>`, "test@example.com")
	if err != nil {
		t.Fatalf("Submit 不应报错: %v", err)
	}
	if repo.created == nil {
		t.Fatal("应创建反馈")
	}
	if strings.Contains(repo.created.Content, "onclick") {
		t.Errorf("入库内容应已 sanitize，got: %s", repo.created.Content)
	}
	if repo.created.UserID != 7 || repo.created.FeedbackType != 2 || repo.created.QuestionID != 0 ||
		repo.created.Contact != "test@example.com" || repo.created.Status != 0 {
		t.Errorf("字段透传不对: %+v", repo.created)
	}
}