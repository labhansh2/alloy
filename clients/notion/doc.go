// Package notion is a minimal client for the Notion REST API (version 2026-03-11).
// This client package is mostly genrated using AI by refering to the API Docs
// Example:
//
//	c := notion.New(os.Getenv("NOTION_TOKEN"), notion.WithHTTPClient(httpClient))
//	page, err := c.GetPage(ctx, pageID, notion.GetPageParams{})
package notion
