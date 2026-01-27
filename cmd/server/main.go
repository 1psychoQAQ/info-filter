package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"info-filter/internal/api"
	"info-filter/internal/fetcher"
	"info-filter/internal/models"
	"info-filter/internal/scorer"
)

func main() {
	// 初始化数据库
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=info_filter port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}
	db.AutoMigrate(&models.Item{}, &models.ItemQuestion{})

	// 初始化评分器
	sc := scorer.NewScorer()

	// 启动定时抓取任务
	go startFetchJob(db, sc)

	// 启动HTTP服务
	r := gin.Default()

	// 查找 web 目录
	webDir := "web"
	// 生产环境：前端独立部署到 frontend 目录
	if _, err := os.Stat("/opt/makestuff/frontend/info-filter"); err == nil {
		webDir = "/opt/makestuff/frontend/info-filter"
	}

	// 加载模板
	r.LoadHTMLGlob(filepath.Join(webDir, "templates/*"))

	// 静态文件
	r.Static("/static", filepath.Join(webDir, "static"))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "info-filter"})
	})

	// 首页 - 渲染 HTML
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "Info Filter",
		})
	})

	// API路由
	handler := api.NewHandler(db, sc)
	handler.RegisterRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	r.Run(":" + port)
}

func startFetchJob(db *gorm.DB, sc *scorer.Scorer) {
	// 启动时先执行一次
	runFetchAndScore(db, sc)

	// 每小时执行一次
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		runFetchAndScore(db, sc)
	}
}

func runFetchAndScore(db *gorm.DB, sc *scorer.Scorer) {
	log.Println("Starting fetch job...")

	fetchers := []fetcher.Fetcher{
		&fetcher.HNFetcher{},
		&fetcher.ProductHuntFetcher{},
		&fetcher.LobstersFetcher{},
		&fetcher.GitHubFetcher{},
	}

	ctx := context.Background()
	threshold := 70

	for _, f := range fetchers {
		log.Printf("Fetching from %s...", f.Name())
		items, err := f.Fetch()
		if err != nil {
			log.Printf("Failed to fetch from %s: %v", f.Name(), err)
			continue
		}

		log.Printf("Got %d items from %s", len(items), f.Name())

		for _, item := range items {
			// 检查是否已存在
			var existing models.Item
			if err := db.Where("url = ?", item.URL).First(&existing).Error; err == nil {
				continue // 已存在，跳过
			}

			// AI评分
			result, err := sc.Score(ctx, item)
			if err != nil {
				log.Printf("Failed to score item %s: %v", item.Title, err)
				continue
			}

			// 填充评分
			item.ScarcityScore = result.Scarcity
			item.ActionableScore = result.Actionable
			item.LeverageScore = result.Leverage
			item.ResonanceScore = result.Resonance
			item.ImportanceScore = result.Importance
			item.BenefitScore = result.Benefit
			item.NobleScore = result.Noble
			item.TotalScore = result.Total
			item.ScoreReason = result.Reason

			// 只保存高分项
			if result.Total >= threshold {
				if err := db.Create(&item).Error; err != nil {
					log.Printf("Failed to save item: %v", err)
				} else {
					log.Printf("[PASS] %s - Score: %d - %s", item.Source, result.Total, item.Title)
				}
			} else {
				log.Printf("[DROP] %s - Score: %d - %s", item.Source, result.Total, item.Title)
			}

			// 避免API限流
			time.Sleep(500 * time.Millisecond)
		}
	}

	log.Println("Fetch job completed")
}
