package handler

import (
	"errors"
	"strconv"

	"software-exam/internal/application/service"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type KnowledgeHandler struct {
	knowledgeService *service.KnowledgeService
}

func NewKnowledgeHandler(knowledgeService *service.KnowledgeService) *KnowledgeHandler {
	return &KnowledgeHandler{knowledgeService: knowledgeService}
}

func (h *KnowledgeHandler) List(c *gin.Context) {
	categoryID, _ := strconv.ParseUint(c.DefaultQuery("category_id", "0"), 10, 64)
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	page, pageSize := parsePage(c)
	kps, total, err := h.knowledgeService.List(categoryID, subjectID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, kps)
}

// ListAdmin GET /api/admin/knowledge —— 后台整树管理专用列表。
// page_size 放宽到 1000：必须一次拉全科目节点。走公开 List 的 50/200 上限会
// 静默截断尾部节点，前端拿残缺快照组树会把丢父级的子节点顶成顶级节点，
// 拖拽提交还会把截断结果固化进库（parent_id 写 0）。
func (h *KnowledgeHandler) ListAdmin(c *gin.Context) {
	categoryID, _ := strconv.ParseUint(c.DefaultQuery("category_id", "0"), 10, 64)
	subjectID, _ := strconv.ParseUint(c.DefaultQuery("subject_id", "0"), 10, 64)
	page, pageSize := parsePageWithMax(c, maxKnowledgePageSize)
	kps, total, err := h.knowledgeService.List(categoryID, subjectID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, kps)
}

func (h *KnowledgeHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	kp, err := h.knowledgeService.GetByID(id)
	if err != nil {
		response.NotFound(c, "知识点不存在")
		return
	}
	response.Success(c, kp)
}

// RelatedQuestions GET /api/knowledge/:id/questions —— 知识点关联真题
func (h *KnowledgeHandler) RelatedQuestions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	list, err := h.knowledgeService.RelatedQuestions(id, limit)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *KnowledgeHandler) Create(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		CategoryID   uint64 `json:"category_id"`
		ParentID     uint64 `json:"parent_id"`
		SubjectID    uint64 `json:"subject_id"`
		Content      string `json:"content"`
		KeyPoints    string `json:"key_points"`
		EasyMistakes string `json:"easy_mistakes"`
		SortOrder    int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	id, err := h.knowledgeService.Create(req.Name, req.CategoryID, req.ParentID, req.SubjectID, req.Content, req.KeyPoints, req.EasyMistakes, req.SortOrder)
	if err != nil {
		h.respondParentError(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

func (h *KnowledgeHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	var req struct {
		Name         string `json:"name"`
		CategoryID   uint64 `json:"category_id"`
		ParentID     uint64 `json:"parent_id"`
		SubjectID    uint64 `json:"subject_id"`
		Content      string `json:"content"`
		KeyPoints    string `json:"key_points"`
		EasyMistakes string `json:"easy_mistakes"`
		SortOrder    int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.knowledgeService.Update(id, req.Name, req.CategoryID, req.ParentID, req.SubjectID, req.Content, req.KeyPoints, req.EasyMistakes, req.SortOrder); err != nil {
		h.respondParentError(c, err)
		return
	}
	response.Success(c, nil)
}

// respondParentError 上级校验/子节点防线的错误映射成 400，其余按服务器错误处理
func (h *KnowledgeHandler) respondParentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrKnowledgeBadParent), errors.Is(err, repository.ErrKnowledgeCycle):
		response.BadRequest(c, "上级节点无效（不存在、跨科目或会造成循环层级），请刷新页面后重试")
	case errors.Is(err, repository.ErrKnowledgeHasChildren):
		response.BadRequest(c, "该节点下仍有子节点，请先删除或移走子项")
	default:
		response.ServerError(c, err.Error())
	}
}

// Reorder PUT /api/admin/knowledge/reorder —— 拖拽排序整树批量提交。
// items 为当前科目的全部节点，每个含新的 parent_id 与同级序号；
// base_version 为提交方加载列表时的 MAX(updated_at)，树在期间被他人改过则拒绝（409）。
func (h *KnowledgeHandler) Reorder(c *gin.Context) {
	var req struct {
		Items       []repository.ReorderItem `json:"items"`
		BaseVersion string                   `json:"base_version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		response.BadRequest(c, "排序列表不能为空")
		return
	}
	version, err := h.knowledgeService.Reorder(req.Items, req.BaseVersion)
	switch {
	case errors.Is(err, repository.ErrReorderConflict):
		response.Error(c, 409, "知识点树刚被其他人修改，已为你保留最新数据，请刷新后重试")
	case errors.Is(err, repository.ErrReorderBadParent), errors.Is(err, repository.ErrReorderCycle):
		response.BadRequest(c, "排序数据无效（父级越界或存在循环），请刷新页面后重试")
	case errors.Is(err, repository.ErrReorderIncomplete):
		// 提交方快照被分页截断（父节点不在批内），拒绝固化截断结果
		response.Error(c, 409, "提交的知识点列表不完整，已拒绝保存以保护层级结构，请刷新页面后重试")
	case err != nil:
		response.ServerError(c, err.Error())
	default:
		response.Success(c, gin.H{"version": version})
	}
}

func (h *KnowledgeHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}
	if err := h.knowledgeService.Delete(id); err != nil {
		h.respondParentError(c, err)
		return
	}
	response.Success(c, nil)
}
