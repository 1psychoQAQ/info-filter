package models

import (
	"time"

	"gorm.io/gorm"
)

type Item struct {
	gorm.Model
	Source      string    `json:"source"`       // HN, ProductHunt, GitHub, RSS
	Title       string    `json:"title"`
	URL         string    `json:"url" gorm:"uniqueIndex"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	PublishedAt time.Time `json:"published_at"`

	// 四维度评分
	ScarcityScore    int `json:"scarcity_score"`    // 稀缺性 0-25
	ActionableScore  int `json:"actionable_score"`  // 可操作性 0-25
	LeverageScore    int `json:"leverage_score"`    // 杠杆率 0-25
	ResonanceScore   int `json:"resonance_score"`   // 人性共鸣 0-25
	TotalScore       int `json:"total_score"`       // 总分 0-100

	// 人性共鸣子分
	ImportanceScore int `json:"importance_score"` // 重要感 0-8
	BenefitScore    int `json:"benefit_score"`    // 利益相关 0-9
	NobleScore      int `json:"noble_score"`      // 高尚动机 0-8

	ScoreReason string `json:"score_reason"` // AI评分理由
	Pushed      bool   `json:"pushed"`       // 是否已推送
}

// ItemQuestion 用户对信息的提问记录
type ItemQuestion struct {
	gorm.Model
	ItemID   uint   `json:"item_id" gorm:"index"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type ScoreResult struct {
	Scarcity   int    `json:"scarcity"`
	Actionable int    `json:"actionable"`
	Leverage   int    `json:"leverage"`
	Resonance  int    `json:"resonance"`
	Importance int    `json:"importance"`
	Benefit    int    `json:"benefit"`
	Noble      int    `json:"noble"`
	Total      int    `json:"total"`
	Reason     string `json:"reason"`
}
