package service

import (
	"errors"
	"strings"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"

	"gorm.io/gorm"
)

func TestSanitizeNoteHTML_StripsEventHandlers(t *testing.T) {
	in := `<p>hello <img src="https://x.cos.ap-shanghai.myqcloud.com/1.png" onerror="alert(1)"> world</p>`
	out := sanitizeNoteHTML(in)
	if strings.Contains(out, "onerror") {
		t.Errorf("expected onerror stripped, got: %s", out)
	}
}

func TestSanitizeNoteHTML_StripsJavascriptProtocol(t *testing.T) {
	in := `<p><a href="javascript:alert(1)">x</a><img src="javascript:alert(1)"></p>`
	out := sanitizeNoteHTML(in)
	if strings.Contains(strings.ToLower(out), "javascript:") {
		t.Errorf("expected javascript: stripped, got: %s", out)
	}
}

func TestSanitizeNoteHTML_AllowsHTTPSImg(t *testing.T) {
	in := `<p><img src="https://bucket.cos.region.myqcloud.com/2026/01/01/foo.png" alt="x"></p>`
	out := sanitizeNoteHTML(in)
	if !strings.Contains(out, `src="https://bucket.cos.region.myqcloud.com/2026/01/01/foo.png"`) {
		t.Errorf("expected https img src preserved, got: %s", out)
	}
}

func TestSanitizeNoteHTML_AllowsDataImage(t *testing.T) {
	in := `<p><img src="data:image/png;base64,iVBORw0KGgo=" alt="x"></p>`
	out := sanitizeNoteHTML(in)
	if !strings.Contains(out, "data:image/png") {
		t.Errorf("expected data:image src preserved, got: %s", out)
	}
}

func TestSanitizeNoteHTML_RejectsOtherImgSchemes(t *testing.T) {
	in := `<p><img src="ftp://x/y.png"><img src="file:///etc/passwd"><img src="vbscript:msgbox(1)"></p>`
	out := sanitizeNoteHTML(in)
	if strings.Contains(out, "ftp://") || strings.Contains(out, "file:///") || strings.Contains(strings.ToLower(out), "vbscript:") {
		t.Errorf("expected non-https/data src cleared, got: %s", out)
	}
}

func TestSanitizeNoteHTML_PreservesPlainText(t *testing.T) {
	in := `这是一段笔记，含 <strong>加粗</strong> 与 <em>斜体</em>。`
	out := sanitizeNoteHTML(in)
	if out != in {
		t.Errorf("expected unchanged, got: %s", out)
	}
}

func TestSanitizeNoteHTML_Empty(t *testing.T) {
	if got := sanitizeNoteHTML(""); got != "" {
		t.Errorf("expected empty, got: %q", got)
	}
}

func TestSanitizeNoteHTML_StripsDataTextHTML(t *testing.T) {
	// data:text/html 是经典 XSS vector
	in := `<p><a href="data:text/html,<script>alert(1)</script>">click</a></p>`
	out := sanitizeNoteHTML(in)
	if strings.Contains(strings.ToLower(out), "data:text/html") {
		t.Errorf("expected data:text/html stripped, got: %s", out)
	}
}

func TestIsSafeImgSrc(t *testing.T) {
	cases := map[string]bool{
		"https://x.cos.region.myqcloud.com/a.png": true,
		"HTTPS://example.com/a.png":                true,
		"data:image/png;base64,abc":                true,
		"data:image/jpeg;base64,abc":               true,
		"javascript:alert(1)":                      false,
		"ftp://x/y":                                false,
		"file:///etc/passwd":                       false,
		"":                                         false,
	}
	for src, want := range cases {
		if got := isSafeImgSrc(src); got != want {
			t.Errorf("isSafeImgSrc(%q) = %v, want %v", src, got, want)
		}
	}
}

// ---- 桩：笔记 repo ----

type stubNoteRepo struct {
	row        *model.NoteWithQuestion
	gotUserID  uint64
	gotQuestID uint64
}

func (s *stubNoteRepo) FindWithQuestion(userID, questionID uint64) (*model.NoteWithQuestion, error) {
	s.gotUserID, s.gotQuestID = userID, questionID
	if s.row == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.row, nil
}

func (s *stubNoteRepo) Upsert(*model.QuestionNote) error { panic("unused") }
func (s *stubNoteRepo) FindByUserAndQuestion(uint64, uint64) (*model.QuestionNote, error) {
	panic("unused")
}
func (s *stubNoteRepo) Delete(uint64, uint64) error { panic("unused") }
func (s *stubNoteRepo) ListByUser(uint64, int, int, string) ([]model.NoteWithQuestion, int64, error) {
	panic("unused")
}

// ---- 桩：题目 repo ----
//
// FindByID 只在「有笔记」的用例里该被调用。q 为 nil 时直接 panic —— 于是
// TestQuestionNoteDetail_NoNote_ReturnsNoteNotFound 顺带断言了一条安全边界：
// 没有笔记时服务层**不会**去读题目表（否则接口就成了能按 id 枚举题库的通道）。

type stubNoteQuestionRepo struct {
	q *model.Question
}

func (s *stubNoteQuestionRepo) FindByID(id uint64) (*model.Question, error) {
	if s.q == nil {
		panic("unused: 没有笔记时不该去读题目")
	}
	return s.q, nil
}

// 接口其余方法本测试都不碰，一律置为不可达。
func (s *stubNoteQuestionRepo) Create(*model.Question) error { panic("unused") }
func (s *stubNoteQuestionRepo) Update(*model.Question) error { panic("unused") }
func (s *stubNoteQuestionRepo) Delete(uint64) error          { panic("unused") }
func (s *stubNoteQuestionRepo) UpdateSortOrders([]uint64) error {
	panic("unused")
}
func (s *stubNoteQuestionRepo) ReplaceOptions(uint64, []model.Option) error {
	panic("unused")
}
func (s *stubNoteQuestionRepo) List(repository.QuestionListReq) ([]model.Question, int64, error) {
	panic("unused")
}
func (s *stubNoteQuestionRepo) FindByIDs([]uint64) ([]model.Question, error) { panic("unused") }
func (s *stubNoteQuestionRepo) GetRandomQuestions(int8, int, string, uint64) ([]model.Question, error) {
	panic("unused")
}
func (s *stubNoteQuestionRepo) GetSeededRandomQuestions(int8, int, string, uint64, uint32) ([]model.Question, error) {
	panic("unused")
}
func (s *stubNoteQuestionRepo) GetRandomByTypeYear(int8, int, uint64, int16) ([]model.Question, error) {
	panic("unused")
}
func (s *stubNoteQuestionRepo) CountQuestions(int8, string, uint64) (int64, error) { panic("unused") }
func (s *stubNoteQuestionRepo) GetCategories(uint64) ([]repository.Category, error) {
	panic("unused")
}

// ---- 用例 ----

func TestQuestionNoteDetail_ReturnsNoteAndQuestion(t *testing.T) {
	noteRepo := &stubNoteRepo{row: &model.NoteWithQuestion{
		ID:         42,
		QuestionID: 12345,
		Content:    "<p>我的笔记</p>",
		PaperTitle: "2025年上半年综合知识",
	}}
	qRepo := &stubNoteQuestionRepo{q: &model.Question{
		ID:      12345,
		Title:   "关于 Bloom 过滤器",
		Content: "完整题干",
		Options: []model.Option{
			{ID: 1, Label: "A", Content: "选项A"},
			{ID: 2, Label: "B", Content: "选项B", IsCorrect: 1},
		},
	}}

	svc := NewQuestionNoteService(noteRepo, qRepo)

	note, question, err := svc.Detail(7, 12345)
	if err != nil {
		t.Fatalf("Detail 返回错误: %v", err)
	}
	if note == nil || note.Content != "<p>我的笔记</p>" || note.PaperTitle != "2025年上半年综合知识" {
		t.Fatalf("note 不对: %+v", note)
	}
	if question == nil || question.ID != 12345 || len(question.Options) != 2 {
		t.Fatalf("question 不对: %+v", question)
	}
	// userID 必须原样透传到 repo —— 丢了它就成了能读别人笔记的越权漏洞
	if noteRepo.gotUserID != 7 || noteRepo.gotQuestID != 12345 {
		t.Fatalf("repo 收到的参数不对: userID=%d questionID=%d", noteRepo.gotUserID, noteRepo.gotQuestID)
	}
}

func TestQuestionNoteDetail_NoNote_ReturnsNoteNotFound(t *testing.T) {
	// qRepo 的 q 为 nil => FindByID 会 panic。这条用例因此同时断言两件事：
	//   1. 没有笔记时报 ErrNoteNotFound
	//   2. 没有笔记时**没有**去读题目表
	noteRepo := &stubNoteRepo{row: nil}
	qRepo := &stubNoteQuestionRepo{q: nil}

	svc := NewQuestionNoteService(noteRepo, qRepo)

	note, question, err := svc.Detail(7, 999)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("期望 ErrNoteNotFound，实际 %v", err)
	}
	if note != nil || question != nil {
		t.Fatalf("失败时不该返回数据: note=%+v question=%+v", note, question)
	}
}
