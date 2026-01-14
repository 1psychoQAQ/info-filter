package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"info-filter/internal/models"
	"info-filter/internal/scorer"
)

type Handler struct {
	db     *gorm.DB
	scorer *scorer.Scorer
}

func NewHandler(db *gorm.DB, sc *scorer.Scorer) *Handler {
	return &Handler{db: db, scorer: sc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/items", h.GetItems)
		api.GET("/items/today", h.GetTodayItems)
		api.GET("/stats", h.GetStats)
		api.POST("/items/:id/ask", h.AskAboutItem)
	}
}

// GetItems 获取高分信息列表
func (h *Handler) GetItems(c *gin.Context) {
	var items []models.Item

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	minScore, _ := strconv.Atoi(c.DefaultQuery("min_score", "70"))
	source := c.Query("source")

	query := h.db.Where("total_score >= ?", minScore).Order("created_at DESC").Limit(limit)
	if source != "" {
		query = query.Where("source = ?", source)
	}

	if err := query.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"count": len(items),
	})
}

// GetTodayItems 获取今日精选
func (h *Handler) GetTodayItems(c *gin.Context) {
	var items []models.Item

	// PostgreSQL 日期比较
	if err := h.db.Where("total_score >= 70").
		Where("created_at >= CURRENT_DATE").
		Order("total_score DESC").
		Limit(50).
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"count": len(items),
	})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	var total, qualified int64
	var avgScore *float64

	h.db.Model(&models.Item{}).Count(&total)
	h.db.Model(&models.Item{}).Where("total_score >= 70").Count(&qualified)
	h.db.Model(&models.Item{}).Select("AVG(total_score)").Scan(&avgScore)

	// 各来源统计
	type SourceStat struct {
		Source string `json:"source"`
		Count  int64  `json:"count"`
	}
	var sourcesStats []SourceStat
	h.db.Model(&models.Item{}).Select("source, count(*) as count").Group("source").Scan(&sourcesStats)

	avg := float64(0)
	if avgScore != nil {
		avg = *avgScore
	}

	passRate := float64(0)
	if total > 0 {
		passRate = float64(qualified) / float64(total) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"qualified": qualified,
		"avg_score": avg,
		"sources":   sourcesStats,
		"pass_rate": passRate,
	})
}

// AskAboutItem 对某条信息进行提问
func (h *Handler) AskAboutItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	var req struct {
		Question string `json:"question" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question is required"})
		return
	}

	// 获取信息
	var item models.Item
	if err := h.db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	// 调用AI回答
	answer, err := h.scorer.Ask(c.Request.Context(), item, req.Question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI error: " + err.Error()})
		return
	}

	// 存入数据库
	question := models.ItemQuestion{
		ItemID:   uint(id),
		Question: req.Question,
		Answer:   answer,
	}
	if err := h.db.Create(&question).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save error: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"answer":      answer,
		"question_id": question.ID,
	})
}
