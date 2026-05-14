package manga

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type MangaDexResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title       map[string]string `json:"title"`
			Description map[string]string `json:"description"`
			Status      string            `json:"status"`
			Tags        []struct {
				Attributes struct {
					Name map[string]string `json:"name"`
				} `json:"attributes"`
			} `json:"tags"`
		} `json:"attributes"`
	} `json:"data"`
}

func FetchFromMangaDex(title string) (Manga, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Manga{}, errors.New("title is required")
	}

	apiURL := "https://api.mangadex.org/manga?limit=1&title=" + url.QueryEscape(title)

	resp, err := http.Get(apiURL)
	if err != nil {
		return Manga{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Manga{}, errors.New("mangadex request failed")
	}

	var result MangaDexResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Manga{}, err
	}
	if len(result.Data) == 0 {
		return Manga{}, errors.New("manga not found on mangadex")
	}

	item := result.Data[0]
	attrs := item.Attributes

	genres := make([]string, 0)
	for _, tag := range attrs.Tags {
		name := pickLocalized(tag.Attributes.Name)
		if name != "" {
			genres = append(genres, name)
		}
	}

	return Manga{
		ID:            item.ID,
		Title:         pickLocalized(attrs.Title),
		Author:        "Unknown",
		Genres:        genres,
		Status:        normalizeStatus(attrs.Status),
		TotalChapters: 0,
		Description:   pickLocalized(attrs.Description),
		Source:        "mangadex",
	}, nil
}

func pickLocalized(values map[string]string) string {
	if value := strings.TrimSpace(values["en"]); value != "" {
		return value
	}

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
}

func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))

	switch status {
	case "ongoing", "completed", "hiatus", "cancelled":
		return status
	default:
		return "unknown"
	}
}
