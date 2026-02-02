package ignore

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

type rule struct {
	base     string
	pattern  string
	neg      bool
	dirOnly  bool
	anchored bool
	hasSlash bool
}

// Matcher evaluates .gitignore rules rooted at a directory.
type Matcher struct {
	root  string
	mu    sync.Mutex
	cache map[string][]rule
}

// NewMatcher creates a matcher rooted at the given directory.
func NewMatcher(root string) *Matcher {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = filepath.Clean(root)
	}
	return &Matcher{root: abs, cache: make(map[string][]rule)}
}

// Ignored reports whether the path should be ignored by .gitignore rules.
func (m *Matcher) Ignored(fullPath string, isDir bool) bool {
	abs, err := filepath.Abs(fullPath)
	if err != nil {
		abs = filepath.Clean(fullPath)
	}
	if abs == m.root {
		return false
	}
	rel, err := filepath.Rel(m.root, abs)
	if err != nil {
		return false
	}
	if strings.HasPrefix(rel, "..") {
		return false
	}
	rel = filepath.ToSlash(rel)
	ignored := applyRules(false, m.rulesForDir(m.root), abs, isDir)
	parts := strings.Split(rel, "/")
	if len(parts) > 1 {
		dir := m.root
		for i := 0; i < len(parts)-1; i++ {
			dir = filepath.Join(dir, parts[i])
			ignored = applyRules(ignored, m.rulesForDir(dir), abs, isDir)
		}
	}
	return ignored
}

func (m *Matcher) rulesForDir(dir string) []rule {
	m.mu.Lock()
	if cached, ok := m.cache[dir]; ok {
		m.mu.Unlock()
		return cached
	}
	m.mu.Unlock()

	rules := parseGitignore(dir)

	m.mu.Lock()
	m.cache[dir] = rules
	m.mu.Unlock()
	return rules
}

func applyRules(ignored bool, rules []rule, fullPath string, isDir bool) bool {
	for _, rule := range rules {
		if rule.matches(fullPath, isDir) {
			ignored = !rule.neg
		}
	}
	return ignored
}

func parseGitignore(dir string) []rule {
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	out := make([]rule, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
			line = line[1:]
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		neg := false
		if strings.HasPrefix(line, "!") {
			neg = true
			line = strings.TrimSpace(line[1:])
			if line == "" {
				continue
			}
		}
		dirOnly := strings.HasSuffix(line, "/")
		if dirOnly {
			line = strings.TrimSuffix(line, "/")
		}
		anchored := strings.HasPrefix(line, "/")
		if anchored {
			line = strings.TrimPrefix(line, "/")
		}
		line = filepath.ToSlash(line)
		if line == "" {
			continue
		}
		out = append(out, rule{
			base:     dir,
			pattern:  line,
			neg:      neg,
			dirOnly:  dirOnly,
			anchored: anchored,
			hasSlash: strings.Contains(line, "/"),
		})
	}
	return out
}

func (r rule) matches(fullPath string, isDir bool) bool {
	if r.dirOnly && !isDir {
		return false
	}
	rel, err := filepath.Rel(r.base, fullPath)
	if err != nil {
		return false
	}
	if strings.HasPrefix(rel, "..") || rel == "." {
		return false
	}
	rel = filepath.ToSlash(rel)
	if !r.hasSlash {
		if r.anchored && strings.Contains(rel, "/") {
			return false
		}
		target := rel
		if !r.anchored {
			target = path.Base(rel)
		}
		ok, err := path.Match(r.pattern, target)
		return err == nil && ok
	}
	ok, err := path.Match(r.pattern, rel)
	return err == nil && ok
}
