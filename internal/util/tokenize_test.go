package util

import "testing"

func TestTokenize(t *testing.T) {
	tokens := Tokenize("Hello, world! Hello.")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0] != "hello" || tokens[1] != "world" || tokens[2] != "hello" {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
}
