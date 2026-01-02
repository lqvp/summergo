package summergo

import (
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nexryai/archer"
)

var errRedditUnsupported = errors.New("unsupported reddit url")

type redditListing struct {
	Kind string `json:"kind"`
	Data struct {
		Children []redditChild `json:"children"`
	} `json:"data"`
}

type redditChild struct {
	Kind string     `json:"kind"`
	Data redditPost `json:"data"`
}

type redditPost struct {
	Title                 string         `json:"title"`
	Selftext              string         `json:"selftext"`
	SubredditNamePrefixed string         `json:"subreddit_name_prefixed"`
	Thumbnail             string         `json:"thumbnail"`
	URLOverriddenByDest   string         `json:"url_overridden_by_dest"`
	Over18                bool           `json:"over_18"`
	PostHint              string         `json:"post_hint"`
	Preview               *redditPreview `json:"preview"`
	Media                 *redditMedia   `json:"media"`
	SecureMedia           *redditMedia   `json:"secure_media"`
}

type redditMedia struct {
	RedditVideo *redditVideo `json:"reddit_video"`
}

type redditVideo struct {
	FallbackURL string `json:"fallback_url"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type redditPreview struct {
	Images []redditImage `json:"images"`
}

type redditImage struct {
	Source redditImageSource `json:"source"`
}

type redditImageSource struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func isRedditHost(host string) bool {
	normalized := strings.ToLower(host)
	return normalized == "reddit.com" || strings.HasSuffix(normalized, ".reddit.com")
}

func buildRedditJSONURL(parsed url.URL) (string, bool) {
	if !isRedditHost(parsed.Hostname()) {
		return "", false
	}
	if !strings.Contains(parsed.Path, "/comments/") {
		return "", false
	}

	if strings.HasSuffix(parsed.Path, ".json") {
		return parsed.String(), true
	}

	jsonURL := parsed
	if strings.HasSuffix(jsonURL.Path, "/") {
		jsonURL.Path = jsonURL.Path + ".json"
	} else {
		jsonURL.Path = jsonURL.Path + "/.json"
	}

	return jsonURL.String(), true
}

func summarizeReddit(siteUrl url.URL) (*Summary, error) {
	jsonURL, ok := buildRedditJSONURL(siteUrl)
	if !ok {
		return nil, errRedditUnsupported
	}

	req, newReqErr := http.NewRequest("GET", jsonURL, nil)
	if newReqErr != nil {
		return nil, fmt.Errorf("failed to create reddit request: %w", newReqErr)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SummerGo/0.1;)")

	requester := archer.SecureRequest{
		Request:     req,
		TimeoutSecs: 10,
		MaxSize:     1024 * 1024 * 10,
	}

	resp, respErr := requester.Send()
	if respErr != nil {
		return nil, respErr
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("non-200 status code: %s", resp.Status)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	var listings []redditListing
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&listings); err != nil {
		return nil, fmt.Errorf("failed to decode reddit json: %w", err)
	}

	return summarizeRedditFromListings(siteUrl, listings)
}

func summarizeRedditFromJSON(siteUrl url.URL, body []byte) (*Summary, error) {
	var listings []redditListing
	if err := json.Unmarshal(body, &listings); err != nil {
		return nil, fmt.Errorf("failed to decode reddit json: %w", err)
	}

	return summarizeRedditFromListings(siteUrl, listings)
}

func summarizeRedditFromListings(siteUrl url.URL, listings []redditListing) (*Summary, error) {
	post, err := redditFindPost(listings)
	if err != nil {
		return nil, err
	}

	return redditSummaryFromPost(siteUrl, post), nil
}

func redditFindPost(listings []redditListing) (redditPost, error) {
	for _, listing := range listings {
		for _, child := range listing.Data.Children {
			if child.Kind == "t3" {
				return child.Data, nil
			}
		}
	}

	return redditPost{}, errors.New("reddit json: post not found")
}

func redditSummaryFromPost(siteUrl url.URL, post redditPost) *Summary {
	siteName := post.SubredditNamePrefixed
	if siteName == "" {
		siteName = "Reddit"
	}

	return &Summary{
		Url:         siteUrl.String(),
		Title:       post.Title,
		Description: post.Selftext,
		Thumbnail:   redditThumbnail(post),
		SiteName:    siteName,
		Icon:        redditIconURL(siteUrl),
		Sensitive:   post.Over18,
		Player:      redditPlayer(post),
	}
}

func redditThumbnail(post redditPost) string {
	if post.Preview != nil && len(post.Preview.Images) > 0 {
		previewURL := strings.TrimSpace(post.Preview.Images[0].Source.URL)
		if previewURL != "" {
			return stdhtml.UnescapeString(previewURL)
		}
	}

	if post.URLOverriddenByDest != "" && (post.PostHint == "image" || strings.HasPrefix(post.URLOverriddenByDest, "https://i.redd.it/")) {
		return post.URLOverriddenByDest
	}

	if strings.HasPrefix(post.Thumbnail, "http://") || strings.HasPrefix(post.Thumbnail, "https://") {
		return post.Thumbnail
	}

	return ""
}

func redditIconURL(siteUrl url.URL) string {
	scheme := siteUrl.Scheme
	if scheme == "" {
		scheme = "https"
	}

	host := siteUrl.Host
	if host == "" {
		host = "www.reddit.com"
	}

	return fmt.Sprintf("%s://%s/favicon.ico", scheme, host)
}

func redditPlayer(post redditPost) Player {
	var player Player

	video := pickRedditVideo(post)
	if video != nil {
		player.Url = video.FallbackURL
		player.Width = video.Width
		player.Height = video.Height
	}

	return player
}

func pickRedditVideo(post redditPost) *redditVideo {
	if post.SecureMedia != nil && post.SecureMedia.RedditVideo != nil {
		return post.SecureMedia.RedditVideo
	}
	if post.Media != nil && post.Media.RedditVideo != nil {
		return post.Media.RedditVideo
	}
	return nil
}
