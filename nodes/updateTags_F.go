package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"alloy"
	"alloy/clients/ai"
	"alloy/clients/notion"
	_ "embed"
)

//go:embed nottager.md
var nottagerPrompt string

const (
	tagModel     = "openai/gpt-4o-mini"
	tagsPropName = "Tags"
)

// UpdateTags classifies General Notes page content and writes multi_select tags.
type UpdateTags struct {
	Notion *notion.Client
	Ai     *ai.Client
	logger *log.Logger

	notesDataSourceID string
}

func (u *UpdateTags) Id() string { return "UpdateTags" }

func (u *UpdateTags) NumInstances() int { return 1 }

func (u *UpdateTags) Init(s alloy.Services) error {
	if u.Notion == nil {
		return errors.New("UpdateTags: notion client is required")
	}
	if u.Ai == nil {
		return errors.New("UpdateTags: ai client is required")
	}
	u.logger = s.Logger
	if s.HttpClient != nil {
		u.Notion.SetHTTPClient(s.HttpClient)
		u.Ai.SetHTTPClient(s.HttpClient)
	}
	return nil
}

func (u *UpdateTags) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, _ chan<- alloy.Job) {
	u.logger.Printf("starting node worker %s", workerId)
	for {
		select {
		case job := <-inJob:
			if err := u.handle(ctx, job); err != nil {
				u.logger.Printf("UpdateTags: %v", err)
			}
		case <-ctx.Done():
			u.logger.Printf("shutting down node worker %s", workerId)
			return
		}
	}
}

func (u *UpdateTags) handle(ctx context.Context, job alloy.Job) error {
	var payload PageTagJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	if payload.PageID == "" {
		return errors.New("missing page_id")
	}

	propName, options, err := u.loadTagOptions(ctx)
	if err != nil {
		return err
	}
	allowed := notion.MultiSelectNames(options)
	if len(allowed) == 0 {
		return errors.New("no tag options on General Notes")
	}

	prompt := buildTagPrompt(allowed, payload.Title, payload.Content)
	temp := 0.0
	resp, err := u.Ai.ChatCompletion(ctx, ai.ChatCompletionParams{
		Model: tagModel,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: &temp,
	})
	if err != nil {
		return fmt.Errorf("chat completion: %w", err)
	}

	tags, err := parseTagResponse(resp.Content(), allowed)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		u.logger.Printf("no valid tags for page %s; skipping update", payload.PageID)
		return nil
	}

	_, err = u.Notion.UpdatePage(ctx, payload.PageID, notion.UpdatePageParams{
		Properties: map[string]any{
			propName: notion.MultiSelectPropertyUpdate(tags),
		},
	})
	if err != nil {
		return fmt.Errorf("update page tags: %w", err)
	}
	u.logger.Printf("updated tags on %s (%q): %v", payload.PageID, payload.Title, tags)
	return nil
}

func (u *UpdateTags) loadTagOptions(ctx context.Context) (string, []notion.MultiSelectOption, error) {
	dsID, err := u.resolveDataSourceID(ctx)
	if err != nil {
		return "", nil, err
	}
	ds, err := u.Notion.GetDataSource(ctx, dsID)
	if err != nil {
		return "", nil, fmt.Errorf("get data source: %w", err)
	}
	name, opts, ok := notion.FindMultiSelectProperty(ds, tagsPropName)
	if !ok {
		return "", nil, errors.New("no multi_select property on General Notes")
	}
	return name, opts, nil
}

func (u *UpdateTags) resolveDataSourceID(ctx context.Context) (string, error) {
	if u.notesDataSourceID != "" {
		return u.notesDataSourceID, nil
	}

	filter, err := json.Marshal(map[string]string{
		"property": "object",
		"value":    "data_source",
	})
	if err != nil {
		return "", err
	}
	list, err := u.Notion.Search(ctx, notion.SearchParams{
		Query:    generalNotesName,
		Filter:   filter,
		PageSize: 20,
	})
	if err != nil {
		return "", err
	}
	for _, raw := range list.Results {
		var obj struct {
			Object string            `json:"object"`
			ID     string            `json:"id"`
			Title  []notion.RichText `json:"title"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil || obj.Object != "data_source" {
			continue
		}
		title := ""
		for _, t := range obj.Title {
			title += t.PlainText
		}
		if strings.EqualFold(strings.TrimSpace(title), generalNotesName) {
			u.notesDataSourceID = obj.ID
			return obj.ID, nil
		}
	}
	return "", fmt.Errorf("could not find data source %q", generalNotesName)
}

func buildTagPrompt(tags []string, title, content string) string {
	p := nottagerPrompt
	p = strings.ReplaceAll(p, "{{TAGS}}", strings.Join(tags, ", "))
	p = strings.ReplaceAll(p, "{{TITLE}}", title)
	p = strings.ReplaceAll(p, "{{CONTENT}}", content)
	return p
}

func parseTagResponse(raw string, allowed []string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil, fmt.Errorf("parse tags %q: %w", raw, err)
	}

	allow := make(map[string]string, len(allowed))
	for _, a := range allowed {
		allow[strings.ToLower(a)] = a
	}

	out := make([]string, 0, len(tags))
	seen := make(map[string]bool)
	for _, t := range tags {
		canon, ok := allow[strings.ToLower(strings.TrimSpace(t))]
		if !ok || seen[canon] {
			continue
		}
		seen[canon] = true
		out = append(out, canon)
		if len(out) == 5 {
			break
		}
	}
	return out, nil
}
