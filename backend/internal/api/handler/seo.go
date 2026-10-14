package handler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"software-exam/config"
	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

// SEOHandler 构建期预渲染的数据出口。
//
// 站点是 SPA，爬虫拿到的只有一个空 div，即使路由改成 history、URL 变好看了，
// 收录到的仍然是空页面。为此在构建期把公开内容（知识点、范文）渲染成静态 HTML
// 随 dist/ 一起发布 —— 爬虫与用户拿到逐字节相同的页面，无 cloaking。
// 后端只负责「数据 + 公开性判定 + 摘要」，markdown 的渲染由构建脚本完成。
// 这里唯一的端点就是给构建脚本用的清单，只覆盖公开内容，题库不在此列。
type SEOHandler struct {
	knowledgeService *service.KnowledgeService
	articleService   *service.ArticleService
}

func NewSEOHandler(knowledgeService *service.KnowledgeService, articleService *service.ArticleService) *SEOHandler {
	return &SEOHandler{knowledgeService: knowledgeService, articleService: articleService}
}

// siteURL 站点对外地址，用于拼绝对 URL。未配置时返回空串。
func siteURL() string {
	return strings.TrimRight(config.AppConfig.Server.SiteURL, "/")
}

// manifestPage 构建期预渲染用的单页数据。
// body 是原始 markdown —— 渲染交给前端构建脚本，后端不管呈现。
type manifestPage struct {
	Path        string `json:"path"`
	Type        string `json:"type"`
	SubjectID   uint64 `json:"subjectId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	H1          string `json:"h1"`
	Body        string `json:"body"`
	UpdatedAt   string `json:"updatedAt"`
}

// knowledgeManifestPage 知识点 → manifest 条目
func knowledgeManifestPage(kp model.KnowledgePoint) manifestPage {
	body := strings.Join([]string{kp.Content, kp.KeyPoints, kp.EasyMistakes}, "\n\n")
	return manifestPage{
		Path:        fmt.Sprintf("/knowledge/%d", kp.ID),
		Type:        "knowledge",
		SubjectID:   kp.SubjectID,
		Title:       kp.Name + " - 软考知识导图 - 软考星图",
		Description: summarize(body, 150),
		H1:          kp.Name,
		Body:        body,
		UpdatedAt:   kp.UpdatedAt.Format("2006-01-02"),
	}
}

// essayManifestPage 范文 → manifest 条目
func essayManifestPage(a model.Article) manifestPage {
	title := a.Title
	if a.Year > 0 {
		title = fmt.Sprintf("%s（%d年）", title, a.Year)
	}
	return manifestPage{
		Path:        fmt.Sprintf("/essays/%d", a.ID),
		Type:        "essay",
		SubjectID:   a.SubjectID,
		Title:       title + " - 软考论文范文 - 软考星图",
		Description: summarize(a.Content, 150),
		H1:          a.Title,
		Body:        a.Content,
		UpdatedAt:   a.UpdatedAt.Format("2006-01-02"),
	}
}

// Manifest GET /api/prerender/manifest
// 构建脚本翻页拉取，直到某页返回空数组为止。total 仅用于进度显示。
// 公开性由仓储层的 List 保证（两处都是 Where("status = 1")），这里不再判 status。
func (h *SEOHandler) Manifest(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "200"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 200
	}

	kps, kpTotal, err := h.knowledgeService.List(0, 0, page, pageSize)
	if err != nil {
		kps = nil
	}
	articles, artTotal, err := h.articleService.List(repository.ArticleListReq{Page: page, PageSize: pageSize})
	if err != nil {
		articles = nil
	}

	pages := make([]manifestPage, 0, len(kps)+len(articles))
	for _, kp := range kps {
		pages = append(pages, knowledgeManifestPage(kp))
	}
	for _, a := range articles {
		pages = append(pages, essayManifestPage(a))
	}

	c.Header("Cache-Control", "public, max-age=3600")
	response.Success(c, gin.H{
		"site":  siteURL(),
		"total": kpTotal + artTotal,
		"pages": pages,
	})
}

// markdownNoise 摘要里要去掉的 markdown 记号与图片占位，避免 description 里出现 `##`、`[图片]`
var markdownNoise = regexp.MustCompile(`!?\[[^\]]*\]\([^)]*\)|\[图片\]|[#*_>` + "`" + `]|\s+`)

// summarize 把正文压成一行纯文本摘要。
// 按 rune 截断 —— 中文按字节切会把字符劈成乱码，搜索结果里直接可见。
func summarize(s string, maxRunes int) string {
	s = strings.TrimSpace(markdownNoise.ReplaceAllString(s, " "))
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
