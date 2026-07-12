package nodes

// PageTagJob is emitted when a General Notes page settles after a significant edit.
type PageTagJob struct {
	PageID  string `json:"page_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}
