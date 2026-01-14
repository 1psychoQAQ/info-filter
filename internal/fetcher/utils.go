package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mmcdole/gofeed"
	"info-filter/internal/models"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func fetchJSON[T any](url string) (T, error) {
	var result T
	resp, err := httpClient.Get(url)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, err
	}
	return result, nil
}

func fetchRSS(source, url string) ([]models.Item, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS %s: %w", url, err)
	}

	items := make([]models.Item, 0, len(feed.Items))
	for _, entry := range feed.Items {
		pubTime := time.Now()
		if entry.PublishedParsed != nil {
			pubTime = *entry.PublishedParsed
		}

		author := ""
		if len(entry.Authors) > 0 {
			author = entry.Authors[0].Name
		}

		items = append(items, models.Item{
			Source:      source,
			Title:       entry.Title,
			URL:         entry.Link,
			Description: entry.Description,
			Author:      author,
			PublishedAt: pubTime,
		})
	}

	return items, nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
