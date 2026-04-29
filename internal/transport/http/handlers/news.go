package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mmcdole/gofeed"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetNews(w http.ResponseWriter, r *http.Request) {
	feed, items, err := getNewsFromRSS()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
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

		imageURLs := newsItemImageURLs(item)
		if len(imageURLs) > 0 {
			rssItem.Image = &imageURLs[0]
			rssItem.Images = imageURLs
		}

		if enclosure := firstNewsImageEnclosure(item); enclosure != "" {
			rssItem.Enclosure = &models.Enclosure{
				URL: &enclosure,
			}
		}

		items = append(items, rssItem)
	}

	return feed, items, nil
}

func newsItemImageURLs(item *gofeed.Item) []string {
	var imageURLs []string

	if item.Image != nil {
		imageURLs = appendNewsImageURL(imageURLs, item.Image.URL)
	}

	for _, enclosure := range item.Enclosures {
		if enclosure == nil {
			continue
		}
		if strings.HasPrefix(enclosure.Type, "image/") || enclosure.Type == "" {
			imageURLs = appendNewsImageURL(imageURLs, enclosure.URL)
		}
	}

	if media, ok := item.Extensions["media"]; ok {
		for _, key := range []string{"content", "thumbnail"} {
			for _, mediaItem := range media[key] {
				imageURLs = appendNewsImageURL(imageURLs, mediaItem.Attrs["url"])
			}
		}
	}

	return imageURLs
}

func firstNewsImageEnclosure(item *gofeed.Item) string {
	for _, enclosure := range item.Enclosures {
		if enclosure == nil {
			continue
		}
		if strings.HasPrefix(enclosure.Type, "image/") || enclosure.Type == "" {
			return strings.TrimSpace(enclosure.URL)
		}
	}
	return ""
}

func appendNewsImageURL(imageURLs []string, rawURL string) []string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return imageURLs
	}

	for _, existingURL := range imageURLs {
		if existingURL == rawURL {
			return imageURLs
		}
	}

	return append(imageURLs, rawURL)
}
