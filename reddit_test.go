package summergo

import (
	"net/url"
	"testing"
)

const redditJSONSample = `[
  {
    "kind": "Listing",
    "data": {
      "children": [
        {
          "kind": "t3",
          "data": {
            "title": "Example title",
            "selftext": "Example body",
            "subreddit_name_prefixed": "r/test",
            "thumbnail": "https://example.com/thumb.jpg",
            "over_18": true,
            "post_hint": "image",
            "url_overridden_by_dest": "https://i.redd.it/example.png",
            "preview": {
              "images": [
                {
                  "source": {
                    "url": "https://preview.redd.it/example.png?width=640&amp;auto=webp",
                    "width": 640,
                    "height": 480
                  }
                }
              ]
            }
          }
        }
      ]
    }
  }
]`

func TestSummarizeRedditFromJSON(t *testing.T) {
	parsed, err := url.Parse("https://www.reddit.com/r/test/comments/abc/example/")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	summary, err := summarizeRedditFromJSON(*parsed, []byte(redditJSONSample))
	if err != nil {
		t.Fatalf("failed to summarize reddit json: %v", err)
	}

	if summary.Title != "Example title" {
		t.Fatalf("unexpected title: %v", summary.Title)
	}
	if summary.Description != "Example body" {
		t.Fatalf("unexpected description: %v", summary.Description)
	}
	if summary.SiteName != "r/test" {
		t.Fatalf("unexpected sitename: %v", summary.SiteName)
	}
	if summary.Thumbnail != "https://preview.redd.it/example.png?width=640&auto=webp" {
		t.Fatalf("unexpected thumbnail: %v", summary.Thumbnail)
	}
	if !summary.Sensitive {
		t.Fatalf("expected sensitive to be true")
	}
}
