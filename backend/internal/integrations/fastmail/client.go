package fastmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/clevercode/aura/internal/db"
)

const sessionURL = "https://api.fastmail.com/.well-known/jmap"

type Config struct {
	Email       string `json:"email"`
	AppPassword string `json:"app_password"`
}

type Client struct {
	cfg     Config
	auth    string
	apiURL  string
	account string
	http    *http.Client
}

type jmapSession struct {
	APIURL      string            `json:"apiUrl"`
	PrimaryAccounts map[string]string `json:"primaryAccounts"`
}

type jmapRequest struct {
	Using       []string        `json:"using"`
	MethodCalls [][]interface{} `json:"methodCalls"`
}

type jmapResponse struct {
	MethodResponses [][]interface{} `json:"methodResponses"`
}

func NewClient(cfg Config) *Client {
	token := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.AppPassword))
	return &Client{
		cfg:  cfg,
		auth: "Basic " + token,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Discover(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("JMAP discover: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized — check your email and app password")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JMAP session returned HTTP %d", resp.StatusCode)
	}

	var session jmapSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return fmt.Errorf("decode JMAP session: %w", err)
	}
	c.apiURL = session.APIURL
	if c.apiURL == "" {
		c.apiURL = "https://api.fastmail.com/jmap/api/"
	}
	c.account = session.PrimaryAccounts["urn:ietf:params:jmap:mail"]
	return nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	return c.Discover(ctx)
}

type Email struct {
	ID          string            `json:"id"`
	Subject     string            `json:"subject"`
	From        []EmailAddress    `json:"from"`
	ReceivedAt  string            `json:"receivedAt"`
}

type EmailAddress struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (c *Client) GetFlaggedEmails(ctx context.Context) ([]Email, error) {
	if c.apiURL == "" {
		if err := c.Discover(ctx); err != nil {
			return nil, err
		}
	}

	body, _ := json.Marshal(jmapRequest{
		Using: []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		MethodCalls: [][]interface{}{
			{"Email/query", map[string]interface{}{
				"accountId": c.account,
				"filter":    map[string]interface{}{"hasKeyword": "$flagged"},
				"limit":     100,
				"sort":      []map[string]interface{}{{"property": "receivedAt", "isAscending": false}},
			}, "0"},
			{"Email/get", map[string]interface{}{
				"accountId": c.account,
				"#ids": map[string]interface{}{
					"resultOf": "0",
					"name":     "Email/query",
					"path":     "/ids/*",
				},
				"properties": []string{"id", "subject", "from", "receivedAt"},
			}, "1"},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JMAP request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JMAP returned HTTP %d", resp.StatusCode)
	}

	var jr jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, err
	}

	// Extract emails from Email/get response (second method call result)
	for _, mc := range jr.MethodResponses {
		if len(mc) < 2 { continue }
		name, ok := mc[0].(string)
		if !ok || name != "Email/get" { continue }

		data, _ := json.Marshal(mc[1])
		var result struct {
			List []Email `json:"list"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result.List, nil
	}
	return nil, nil
}

// Sync fetches starred/flagged Fastmail emails and creates tasks.
func Sync(ctx context.Context, cfg Config, tasks *db.TaskStore) (db.SyncResult, error) {
	client := NewClient(cfg)
	emails, err := client.GetFlaggedEmails(ctx)
	if err != nil {
		return db.SyncResult{}, err
	}

	var result db.SyncResult
	for _, em := range emails {
		if err := syncEmail(ctx, em, cfg.Email, tasks, &result); err != nil {
			result.Errors++
		}
	}
	return result, nil
}

func syncEmail(ctx context.Context, em Email, accountEmail string, tasks *db.TaskStore, result *db.SyncResult) error {
	result.Total++

	_, err := tasks.FindBySource(ctx, "fastmail", em.ID)
	if err == nil {
		return nil // already imported
	}
	if !errors.Is(err, db.ErrNotFound) {
		return err
	}

	subject := em.Subject
	if subject == "" {
		subject = "(no subject)"
	}

	fromStr := ""
	if len(em.From) > 0 {
		f := em.From[0]
		if f.Name != "" {
			fromStr = f.Name + " <" + f.Email + ">"
		} else {
			fromStr = f.Email
		}
	}

	meta, _ := json.Marshal(map[string]string{
		"from": fromStr,
		"date": em.ReceivedAt,
	})
	metaStr := string(meta)
	source := "fastmail"
	sourceURL := "https://app.fastmail.com/mail/"

	_, createErr := tasks.Create(ctx, db.CreateTaskParams{
		ID:             uuid.New().String(),
		Title:          subject,
		Status:         "backlog",
		Position:       float64(time.Now().UnixMilli()),
		Source:         &source,
		SourceID:       &em.ID,
		SourceURL:      &sourceURL,
		SourceMetadata: &metaStr,
	})
	if createErr != nil {
		return createErr
	}
	result.New++
	return nil
}
