package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const DefaultJQL = `assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC`

type Config struct {
	Host     string `json:"host"`
	Email    string `json:"email"`
	APIToken string `json:"api_token"`
	JQL      string `json:"jql"`
}

type IssueFields struct {
	Summary   string     `json:"summary"`
	Status    Status     `json:"status"`
	Priority  *Priority  `json:"priority"`
	IssueType IssueType  `json:"issuetype"`
}

type Status struct {
	Name           string         `json:"name"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

type StatusCategory struct {
	Key string `json:"key"` // "new", "indeterminate", "done"
}

type Priority struct {
	Name string `json:"name"`
}

type IssueType struct {
	Name string `json:"name"`
}

type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

type SearchResult struct {
	Issues     []Issue `json:"issues"`
	Total      int     `json:"total"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
}

type Client struct {
	cfg  Config
	auth string
	http *http.Client
}

func NewClient(cfg Config) *Client {
	token := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	return &Client{
		cfg:  cfg,
		auth: "Basic " + token,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Search(ctx context.Context, startAt, maxResults int) (SearchResult, error) {
	jql := c.cfg.JQL
	if jql == "" {
		jql = DefaultJQL
	}

	params := url.Values{}
	params.Set("jql", jql)
	params.Set("startAt", fmt.Sprintf("%d", startAt))
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	params.Set("fields", "summary,status,priority,issuetype")

	reqURL := c.cfg.Host + "/rest/api/3/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return SearchResult{}, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return SearchResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return SearchResult{}, fmt.Errorf("unauthorized — check your email and API token")
	}
	if resp.StatusCode != http.StatusOK {
		return SearchResult{}, fmt.Errorf("jira returned HTTP %d", resp.StatusCode)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SearchResult{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.Search(ctx, 0, 1)
	return err
}
