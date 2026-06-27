package types

// Ticket represents a support ticket.
type Ticket struct {
	ID        string         `json:"id"`
	Subject   string         `json:"subject"`
	Status    TicketStatus   `json:"status"`
	Priority  TicketPriority `json:"priority"`
	Body      string         `json:"body"`
	UpdatedAt string         `json:"updatedAt"`
}

// TicketInput is the write model for ticket create/update.
type TicketInput struct {
	Subject  string         `json:"subject,omitempty"`
	Priority TicketPriority `json:"priority,omitempty"`
	Body     string         `json:"body,omitempty"`
}

// DocArticle represents a documentation article.
type DocArticle struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
}
