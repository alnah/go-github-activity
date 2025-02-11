package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
)

type (
	// ghEvent represents a GitHub ghEvent
	ghEvent struct {
		ID        string    `json:"id"`
		Type      string    `json:"type"`
		Actor     actor     `json:"actor"`
		Repo      repo      `json:"repo"`
		Payload   payload   `json:"payload"`
		Public    bool      `json:"public"`
		CreatedAt time.Time `json:"created_at"`
	}
	// actor represents the user who triggered the event
	actor struct {
		ID           int    `json:"id"`
		Login        string `json:"login"`
		DisplayLogin string `json:"display_login"`
		URL          string `json:"url"`
	}
	// repo represents the repository involved in the event
	repo struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	// payload represents the specific data related to the event
	payload struct {
		Action       string   `json:"action,omitempty"`
		PushID       int64    `json:"push_id,omitempty"`
		Size         int      `json:"size,omitempty"`
		DistinctSize int      `json:"distinct_size,omitempty"`
		Ref          string   `json:"ref,omitempty"`
		Head         string   `json:"head,omitempty"`
		Before       string   `json:"before,omitempty"`
		Commits      []commit `json:"commits,omitempty"`
	}
	// commit represents a commit in a push event
	commit struct {
		SHA      string `json:"sha"`
		Author   author `json:"author"`
		Message  string `json:"message"`
		Distinct bool   `json:"distinct"`
		URL      string `json:"url"`
	}
	// author represents the author of a commit
	author struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	// client manages authenticated requests and error handling for GitHub API.
	client struct {
		url    string
		Token  string
		Method string
		Client *http.Client
	}
)

// newClient configures secure defaults for GitHub API communication.
func newClient(token string) *client {
	return &client{
		Token:  token,
		Method: "GET",
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *client) setURL(url string) {
	c.url = url
}

// fetchGitHubResponse gets a single page of results from GitHub API.
func fetchGitHubResponse(hc *client, url string) (ghEvent, error) {
	hc.setURL(url)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ghRes, err := hc.do(ctx)
	if err != nil {
		return ghEvent{}, err
	}
	return ghRes, nil
}

// do retrieves movie data from GitHub with a retry mechanism based on exponential backoff.
func (hc *client) do(ctx context.Context) (ghEvent, error) {
	op := func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, hc.Method, hc.url, nil)
		if err != nil {
			return nil, backoff.Permanent(fmt.Errorf("request error: %w", err))
		}
		req.Header.Add("Authorization", "Bearer "+hc.APIKey)
		req.Header.Add("Content-Type", "application/json")
		cli := newClient(hc.APIKey)
		res, err := cli.Client.Do(req)
		if err != nil {
			return nil, backoff.Permanent(fmt.Errorf("request error: %w", err))
		}
		switch {
		case res.StatusCode >= 500:
			return nil, backoff.Permanent(fmt.Errorf("GitHub API server error: %q", res.Status))
		case res.StatusCode == 429:
			sec, err := strconv.ParseInt(res.Header.Get("Retry-After"), 10, 64)
			if err == nil {
				return nil, backoff.RetryAfter(int(sec))
			}
		case res.StatusCode >= 400:
			return nil, backoff.Permanent(fmt.Errorf("GitHub API client error: %q", res.Status))
		}
		return res, nil
	}
	res, err := backoff.Retry(ctx, op, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		return ghEvent{}, fmt.Errorf("fetch GitHub response: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()
	var results ghEvent
	if err = json.NewDecoder(res.Body).Decode(&results); err != nil {
		return ghEvent{}, fmt.Errorf("decode response: %w", err)
	}
	return results, nil
}
