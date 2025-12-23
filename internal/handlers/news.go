package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mmcdole/gofeed"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetNews(w http.ResponseWriter, r *http.Request) {
	feed, items, err := getNewsFromRSS()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := models.NewsAPIResponse{
		Success: true,
		Data: models.NewsAPIData{
			Title:       feed.Title,
			Description: feed.Description,
			Link:        feed.Link,
			Items:       items,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// getNewsFromRSS fetches the RSS feed and maps it to RSSItemResponse models.
func getNewsFromRSS() (*gofeed.Feed, []models.RSSItemResponse, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL("https://www.espncricinfo.com/rss/content/story/feeds/0.xml")
	if err != nil {
		return nil, nil, err
	}

	var items []models.RSSItemResponse
	for _, item := range feed.Items {
		content := item.Content
		if content == "" {
			content = item.Description
		}

		rssItem := models.RSSItemResponse{
			Title:          &item.Title,
			Link:           &item.Link,
			PubDate:        &item.Published,
			ContentSnippet: &item.Description,
			Content:        &content,
			GUID:           &item.GUID,
		}

		if len(item.Enclosures) > 0 {
			url := item.Enclosures[0].URL
			rssItem.Enclosure = &models.Enclosure{
				URL: &url,
			}
			rssItem.Image = &url
		}

		items = append(items, rssItem)
	}

	return feed, items, nil
}
