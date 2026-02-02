package search

import "testing"

func TestSanitizeFTSQuery(t *testing.T) {
	query := `foo:"bar baz" +qux*`
	got := sanitizeFTSQuery(query)
	if got != "foo bar baz qux" {
		t.Fatalf("unexpected sanitized query: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("unexpected truncate result: %q", got)
	}
	if got := truncate("1234567890", 5); got != "12345..." {
		t.Fatalf("unexpected truncate result: %q", got)
	}
}

func TestIsFTSSyntaxError(t *testing.T) {
	if !isFTSSyntaxError(errString("fts5: syntax error near \"-\"")) {
		t.Fatalf("expected syntax error detection")
	}
	if !isFTSSyntaxError(errString("SQL logic error: no such column: f (1)")) {
		t.Fatalf("expected no such column detection")
	}
	if isFTSSyntaxError(errString("some other error")) {
		t.Fatalf("unexpected syntax error detection")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
