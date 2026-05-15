package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"mygrep/internal/domain"
)

type Client struct {
	id      string
	baseURL string
	http    *http.Client
}

func NewClient(id, baseURL string, timeout time.Duration) *Client {
	return &Client{
		id:      id,
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) ID() string { return c.id }

func (c *Client) Submit(ctx context.Context, chunk domain.Chunk, pattern string) ([]domain.Match, error) {
	body := ProcessRequest{
		ChunkID:   chunk.ID,
		Source:    chunk.Source,
		StartLine: chunk.StartLine,
		Lines:     chunk.Lines,
		Pattern:   pattern,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/process", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worker %s: %w", c.id, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var pr ProcessResponse
	if jerr := json.Unmarshal(data, &pr); jerr != nil {
		return nil, fmt.Errorf("worker %s: bad response (status=%d): %s", c.id, resp.StatusCode, string(data))
	}
	if pr.Error != "" {
		return nil, errors.New(pr.Error)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("worker %s: http %d", c.id, resp.StatusCode)
	}

	out := make([]domain.Match, 0, len(pr.Matches))
	for _, d := range pr.Matches {
		out = append(out, fromDTO(d))
	}
	return out, nil
}
