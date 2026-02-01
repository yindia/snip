package search

type Result struct {
	DocID      string  `json:"docid"`
	Collection string  `json:"collection"`
	RelPath    string  `json:"relpath"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Snippet    string  `json:"snippet"`
	Context    string  `json:"context"`
}

type Options struct {
	Limit      int
	Collection string
	All        bool
	MinScore   float64
	Full       bool
}

type Expander interface {
	Expand(query string) ([]string, error)
}
