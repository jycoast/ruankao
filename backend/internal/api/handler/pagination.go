package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// 分页与批量参数硬上限：防止通过超大 page_size / count 一次性导出题库
const (
	maxPageSize          = 50
	maxRandomCount       = 50
	maxAdminPageSize     = 200 // 后台维护需要更大翻页，但仍需上限防止误拉全表
	maxKnowledgePageSize = 1000 // 知识点整树管理/导图必须一次拉全科目（单科目约 200+ 节点）
)

// parsePage 解析并钳制分页参数：page ≥ 1，1 ≤ page_size ≤ maxPageSize。
// 管理员放宽到 maxAdminPageSize —— 50 的上限是为防爬，对已鉴权的后台反而
// 让超过 50 题的分组后半截永远打不开（爬来的错题因此无法修正）。
func parsePage(c *gin.Context) (page, pageSize int) {
	limit := maxPageSize
	if role, ok := c.Get("role"); ok {
		if r, ok := role.(int8); ok && r == 1 {
			limit = maxAdminPageSize
		}
	}
	return parsePageWithMax(c, limit)
}

// parsePageWithMax 同 parsePage，但 page_size 上限由调用方指定。
// 知识点整树场景必须用它：公开接口 50/200 的上限会静默截断尾部节点，
// 前端拿残缺快照组树会把丢父级的子节点顶成顶级节点。
func parsePageWithMax(c *gin.Context, limit int) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > limit {
		pageSize = limit
	}
	return page, pageSize
}

// clampCount 钳制批量数量参数（如随机抽题的 count）。
func clampCount(count, def, max int) int {
	if count < 1 {
		return def
	}
	if count > max {
		return max
	}
	return count
}
