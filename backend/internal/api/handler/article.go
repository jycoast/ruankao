package handler

import (
	"software-exam/internal/application/service"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ArticleHandler struct {
	articleService *service.ArticleService
}

func NewArticleHandler(articleService *service.ArticleService) *ArticleHandler {
	return &ArticleHandler{articleService: articleService}
}

func (h *ArticleHandler) List(c *gin.Context) {
	articleType, _ := strconv.Atoi(c.DefaultQuery("article_type", "0"))
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	page, pageSize := parsePage(c)
	articles, total, err := h.articleService.List(repository.ArticleListReq{
		ArticleType: int8(articleType),
		SubjectID:   subjectID,
		Page:        page,
		PageSize:    pageSize,
		Keyword:     c.Query("keyword"),
		Category:    c.Query("category"),
	})
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, articles)
}

func (h *ArticleHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	article, err := h.articleService.GetByID(id)
	if err != nil {
		response.NotFound(c, "文章不存在")
		return
	}
	response.Success(c, article)
}

func (h *ArticleHandler) Create(c *gin.Context) {
	var req struct {
		Title       string  `json:"title" binding:"required"`
		Content     string  `json:"content" binding:"required"`
		ArticleType int8    `json:"article_type" binding:"required"`
		Category    string  `json:"category"`
		Year        int16   `json:"year"`
		Score       float32 `json:"score"`
		Author      string  `json:"author"`
		Tags        string  `json:"tags"`
		SubjectID   uint64  `json:"subject_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	id, err := h.articleService.Create(req.Title, req.Content, req.ArticleType, req.Category, req.Year, req.Score, req.Author, req.Tags, req.SubjectID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"id": id})
}

func (h *ArticleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Title       string  `json:"title"`
		Content     string  `json:"content"`
		ArticleType int8    `json:"article_type"`
		Category    string  `json:"category"`
		Year        int16   `json:"year"`
		Score       float32 `json:"score"`
		Author      string  `json:"author"`
		Tags        string  `json:"tags"`
		SubjectID   uint64  `json:"subject_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.articleService.Update(id, req.Title, req.Content, req.ArticleType, req.Category, req.Year, req.Score, req.Author, req.Tags, req.SubjectID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *ArticleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	if err := h.articleService.Delete(id); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}
