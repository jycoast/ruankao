package handler

import (
	"strings"
	"testing"
	"time"

	"software-exam/internal/domain/model"
)

// 中文摘要必须按 rune 截断。按字节切会把一个汉字劈成两半，
// 乱码直接出现在搜索结果的 description 里。
func TestSummarizeTruncatesByRune(t *testing.T) {
	got := summarize(strings.Repeat("架", 200), 10)
	if got != strings.Repeat("架", 10)+"…" {
		t.Fatalf("expected 10 runes + ellipsis, got %q", got)
	}
}

func TestSummarizeStripsMarkdownNoise(t *testing.T) {
	got := summarize("## 标题\n\n**重点**：见 ![图](a.png) 与 [图片]", 100)
	for _, bad := range []string{"#", "*", "![", "[图片]", "\n"} {
		if strings.Contains(got, bad) {
			t.Fatalf("summary should not contain %q, got %q", bad, got)
		}
	}
	if !strings.Contains(got, "标题") || !strings.Contains(got, "重点") {
		t.Fatalf("summary lost real content: %q", got)
	}
}

func TestSummarizeShortTextUnchanged(t *testing.T) {
	if got := summarize("软件架构设计", 100); got != "软件架构设计" {
		t.Fatalf("short text should pass through, got %q", got)
	}
}

func TestKnowledgeManifestPageShape(t *testing.T) {
	kp := model.KnowledgePoint{
		ID: 310, SubjectID: 1, Name: "开放-封闭原则",
		Content:   "## 定义\n\n软件实体应该对扩展开放。",
		KeyPoints: "**OCP**",
		UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	got := knowledgeManifestPage(kp)

	if got.Path != "/knowledge/310" {
		t.Fatalf("path = %q", got.Path)
	}
	if got.Type != "knowledge" {
		t.Fatalf("type = %q", got.Type)
	}
	if got.Title != "开放-封闭原则 - 软考知识导图 - 软考星图" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.H1 != "开放-封闭原则" {
		t.Fatalf("h1 = %q", got.H1)
	}
	if got.SubjectID != 1 {
		t.Fatalf("subjectId = %d", got.SubjectID)
	}
	if got.UpdatedAt != "2026-01-02" {
		t.Fatalf("updatedAt = %q", got.UpdatedAt)
	}
	// body 必须是原始 markdown 交给前端渲染，不能被转成 HTML
	if !strings.Contains(got.Body, "## 定义") {
		t.Fatalf("body 应是原始 markdown, got %q", got.Body)
	}
	// description 是纯文本摘要，不能带 markdown 记号
	for _, bad := range []string{"#", "*", "\n"} {
		if strings.Contains(got.Description, bad) {
			t.Fatalf("description 含 %q: %q", bad, got.Description)
		}
	}
}

func TestEssayManifestPageIncludesYearInTitle(t *testing.T) {
	a := model.Article{
		ID: 3, SubjectID: 1, Title: "论软件架构风格", Year: 2023,
		Content:   "本文论述了架构风格的选择。",
		UpdatedAt: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
	}
	got := essayManifestPage(a)

	if got.Path != "/essays/3" {
		t.Fatalf("path = %q", got.Path)
	}
	if got.Type != "essay" {
		t.Fatalf("type = %q", got.Type)
	}
	if got.Title != "论软件架构风格（2023年） - 软考论文范文 - 软考星图" {
		t.Fatalf("title = %q", got.Title)
	}
	// h1 用不带年份的原标题，年份只进 title
	if got.H1 != "论软件架构风格" {
		t.Fatalf("h1 = %q", got.H1)
	}
}

func TestEssayManifestPageOmitsYearWhenZero(t *testing.T) {
	got := essayManifestPage(model.Article{ID: 9, Title: "无年份范文", Content: "正文"})
	if got.Title != "无年份范文 - 软考论文范文 - 软考星图" {
		t.Fatalf("title = %q", got.Title)
	}
}
