package fetcher

import (
	"info-filter/internal/models"
	"time"
)

type Fetcher interface {
	Name() string
	Fetch() ([]models.Item, error)
}

// HackerNews fetcher
type HNFetcher struct{}

func (f *HNFetcher) Name() string { return "HackerNews" }

func (f *HNFetcher) Fetch() ([]models.Item, error) {
	// HN API: https://hacker-news.firebaseio.com/v0/topstories.json
	// 获取top 30条
	items := make([]models.Item, 0)

	resp, err := fetchJSON[[]int]("https://hacker-news.firebaseio.com/v0/topstories.json")
	if err != nil {
		return nil, err
	}

	// 只取前30条
	limit := 30
	if len(resp) < limit {
		limit = len(resp)
	}

	for _, id := range resp[:limit] {
		item, err := fetchHNItem(id)
		if err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

type HNItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	By    string `json:"by"`
	Time  int64  `json:"time"`
	Text  string `json:"text"`
}

func fetchHNItem(id int) (models.Item, error) {
	url := "https://hacker-news.firebaseio.com/v0/item/" + itoa(id) + ".json"
	hn, err := fetchJSON[HNItem](url)
	if err != nil {
		return models.Item{}, err
	}

	itemURL := hn.URL
	if itemURL == "" {
		itemURL = "https://news.ycombinator.com/item?id=" + itoa(hn.ID)
	}

	return models.Item{
		Source:      "HackerNews",
		Title:       hn.Title,
		URL:         itemURL,
		Description: hn.Text,
		Author:      hn.By,
		PublishedAt: time.Unix(hn.Time, 0),
	}, nil
}

// ProductHunt fetcher (RSS方式，免费)
type ProductHuntFetcher struct{}

func (f *ProductHuntFetcher) Name() string { return "ProductHunt" }

func (f *ProductHuntFetcher) Fetch() ([]models.Item, error) {
	return fetchRSS("ProductHunt", "https://www.producthunt.com/feed")
}

// Lobsters fetcher
type LobstersFetcher struct{}

func (f *LobstersFetcher) Name() string { return "Lobsters" }

func (f *LobstersFetcher) Fetch() ([]models.Item, error) {
	return fetchRSS("Lobsters", "https://lobste.rs/rss")
}

// GitHub Trending (简化版，抓取页面)
type GitHubFetcher struct{}

func (f *GitHubFetcher) Name() string { return "GitHub" }

func (f *GitHubFetcher) Fetch() ([]models.Item, error) {
	// 使用非官方API
	return fetchRSS("GitHub", "https://mshibanern.github.io/GitHubTrendingRSS/daily/all.xml")
}

// 通用RSS fetcher
type RSSFetcher struct {
	SourceName string
	FeedURL    string
}

func (f *RSSFetcher) Name() string { return f.SourceName }

func (f *RSSFetcher) Fetch() ([]models.Item, error) {
	return fetchRSS(f.SourceName, f.FeedURL)
}
