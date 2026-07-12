package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"alloy"
	"alloy/clients/notion"
)

const (
	pollInterval     = 5 * time.Second
	idleTimeout      = 45 * time.Second
	generalNotesName = "General Notes"
)

type dirtyPage struct {
	lastSeen   string
	lastChange time.Time
}

// DetectPageUpdate watches Notion page webhooks and emits tag jobs as soon as
// a significant content change is seen. Dirty pages are still polled briefly so
// Notion lag / follow-up edits are caught; idle only stops watching.
type DetectPageUpdate struct {
	Notion *notion.Client
	logger *log.Logger

	dirty   map[string]*dirtyPage
	settled map[string]string

	notesDataSourceID string
	notesDatabaseID   string
	notesPageID       string
}

func (d *DetectPageUpdate) Id() string { return "DetectPageUpdate" }

func (d *DetectPageUpdate) NumInstances() int { return 1 }

func (d *DetectPageUpdate) Init(s alloy.Services) error {
	if d.Notion == nil {
		return errors.New("DetectPageUpdate: notion client is required")
	}
	d.logger = s.Logger
	d.dirty = make(map[string]*dirtyPage)
	d.settled = make(map[string]string)
	if s.HttpClient != nil {
		d.Notion.SetHTTPClient(s.HttpClient)
	}
	return nil
}

func (d *DetectPageUpdate) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, outJob chan<- alloy.Job) {
	d.logger.Printf("starting node worker %s", workerId)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case data := <-inJob:
			d.onEvent(ctx, data, outJob)
		case <-ticker.C:
			d.pollDirty(ctx, outJob)
		case <-ctx.Done():
			d.logger.Printf("shutting down node worker %s", workerId)
			return
		}
	}
}

func (d *DetectPageUpdate) onEvent(ctx context.Context, data alloy.Job, outJob chan<- alloy.Job) {
	var event notion.WebhookPayload
	if err := json.Unmarshal(data.Payload, &event); err != nil {
		d.logger.Printf("invalid notion event: %v", err)
		return
	}
	if event.Entity.Type != "page" {
		return
	}
	if event.Type == notion.EventPagePropertiesUpdated {
		return
	}

	pageID := event.Entity.Id
	md, err := d.Notion.RetrievePageMarkdown(ctx, pageID)
	if err != nil {
		d.logger.Printf("failed to retrieve page %s: %v", pageID, err)
		return
	}

	now := time.Now()
	if entry, ok := d.dirty[pageID]; ok {
		if md.Markdown != entry.lastSeen {
			entry.lastSeen = md.Markdown
			d.logger.Printf("page %s changed while dirty", pageID)
		}
		entry.lastChange = now
	} else {
		d.dirty[pageID] = &dirtyPage{
			lastSeen:   md.Markdown,
			lastChange: now,
		}
		d.logger.Printf("watching dirty page %s", pageID)
	}

	d.maybeEmitTagJob(ctx, pageID, md.Markdown, outJob)
}

func (d *DetectPageUpdate) pollDirty(ctx context.Context, outJob chan<- alloy.Job) {
	now := time.Now()
	for pageID, entry := range d.dirty {
		md, err := d.Notion.RetrievePageMarkdown(ctx, pageID)
		if err != nil {
			d.logger.Printf("poll page %s: %v", pageID, err)
			continue
		}
		if md.Markdown != entry.lastSeen {
			entry.lastSeen = md.Markdown
			entry.lastChange = now
			d.logger.Printf("page %s changed on poll", pageID)
			d.maybeEmitTagJob(ctx, pageID, entry.lastSeen, outJob)
			continue
		}

		if now.Sub(entry.lastChange) >= idleTimeout {
			delete(d.dirty, pageID)
			d.logger.Printf("page %s idle; stopped polling", pageID)
		}
	}
}

func (d *DetectPageUpdate) maybeEmitTagJob(ctx context.Context, pageID, content string, outJob chan<- alloy.Job) {
	if containsBangAI(content) {
		d.logger.Printf("page %s contains !ai; skipping tag flow", pageID)
		return
	}
	if !significantChange(d.settled[pageID], content) {
		return
	}

	page, err := d.Notion.GetPage(ctx, pageID, notion.GetPageParams{})
	if err != nil {
		d.logger.Printf("get page %s: %v", pageID, err)
		return
	}
	ok, err := d.isGeneralNotesPage(ctx, page)
	if err != nil {
		d.logger.Printf("general notes check %s: %v", pageID, err)
		return
	}
	if !ok {
		d.logger.Printf("page %s not under %s; skipping", pageID, generalNotesName)
		return
	}

	job := PageTagJob{
		PageID:  pageID,
		Title:   notion.PageTitle(page),
		Content: content,
	}
	payload, err := json.Marshal(job)
	if err != nil {
		d.logger.Printf("marshal tag job: %v", err)
		return
	}
	d.settled[pageID] = content
	outJob <- alloy.Job{Source: d.Id(), Payload: payload}
	d.logger.Printf("emitted tag job for page %s (%q)", pageID, job.Title)
}

func (d *DetectPageUpdate) isGeneralNotesPage(ctx context.Context, page *notion.Page) (bool, error) {
	if err := d.resolveGeneralNotes(ctx); err != nil {
		return false, err
	}

	switch page.Parent.Type {
	case "data_source_id":
		return d.notesDataSourceID != "" && page.Parent.DataSourceID == d.notesDataSourceID, nil
	case "database_id":
		return d.notesDatabaseID != "" && page.Parent.DatabaseID == d.notesDatabaseID, nil
	case "page_id":
		if d.notesPageID != "" && page.Parent.PageID == d.notesPageID {
			return true, nil
		}
		parent, err := d.Notion.GetPage(ctx, page.Parent.PageID, notion.GetPageParams{})
		if err != nil {
			return false, err
		}
		return strings.EqualFold(notion.PageTitle(parent), generalNotesName), nil
	default:
		return false, nil
	}
}

func (d *DetectPageUpdate) resolveGeneralNotes(ctx context.Context) error {
	if d.notesDataSourceID != "" || d.notesDatabaseID != "" || d.notesPageID != "" {
		return nil
	}

	if err := d.searchGeneralNotes(ctx, true); err == nil {
		return nil
	}
	return d.searchGeneralNotes(ctx, false)
}

func (d *DetectPageUpdate) searchGeneralNotes(ctx context.Context, dataSourcesOnly bool) error {
	params := notion.SearchParams{
		Query:    generalNotesName,
		PageSize: 20,
	}
	if dataSourcesOnly {
		filter, err := json.Marshal(map[string]string{
			"property": "object",
			"value":    "data_source",
		})
		if err != nil {
			return err
		}
		params.Filter = filter
	}

	list, err := d.Notion.Search(ctx, params)
	if err != nil {
		return err
	}

	for _, raw := range list.Results {
		var obj struct {
			Object     string                     `json:"object"`
			ID         string                     `json:"id"`
			Title      []notion.RichText          `json:"title"`
			Properties map[string]notion.Property `json:"properties"`
			Parent     notion.Parent              `json:"parent"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}

		title := ""
		for _, t := range obj.Title {
			title += t.PlainText
		}
		if title == "" && obj.Object == "page" {
			title = notion.PageTitle(&notion.Page{Properties: obj.Properties})
		}
		if !strings.EqualFold(strings.TrimSpace(title), generalNotesName) {
			continue
		}

		switch obj.Object {
		case "data_source":
			d.notesDataSourceID = obj.ID
			if obj.Parent.Type == "database_id" {
				d.notesDatabaseID = obj.Parent.DatabaseID
			}
			d.logger.Printf("resolved %s data source %s", generalNotesName, obj.ID)
			return nil
		case "database":
			d.notesDatabaseID = obj.ID
			d.logger.Printf("resolved %s database %s", generalNotesName, obj.ID)
			return nil
		case "page":
			d.notesPageID = obj.ID
			d.logger.Printf("resolved %s page %s", generalNotesName, obj.ID)
			return nil
		}
	}

	return errors.New("could not resolve General Notes in Notion search")
}
