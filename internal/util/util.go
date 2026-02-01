package util

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var headingRe = regexp.MustCompile(`^#{1,6}\s+(.+)$`)

// HashContent returns a hex-encoded SHA256 hash of the content.
func HashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// DocIDFromHash returns a stable short docid derived from a content hash.
func DocIDFromHash(hash string) string {
	if len(hash) < 6 {
		return hash
	}
	return hash[:6]
}

// TitleFromMarkdown extracts the first Markdown heading or falls back to filename.
func TitleFromMarkdown(content string, filename string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := headingRe.FindStringSubmatch(line)
		if len(m) == 2 {
			return strings.TrimSpace(m[1])
		}
	}
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		return base
	}
	return name
}

// CacheDir returns the base cache directory for snip.
func CacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v
	}
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "snip-cache")
}

// CleanAbs returns a cleaned absolute path.
func CleanAbs(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// Tokenize splits text into lowercase tokens.
func Tokenize(text string) []string {
	fields := strings.Fields(strings.ToLower(text))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(strings.Trim(f, ",.;:!?()[]{}\"'`"))
		if f == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}
