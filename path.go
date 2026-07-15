package httpexporter

import (
	"regexp"
	"strings"
	"sync"
)

const slugCacheCap = 10_000

var (
	slugCache   = make(map[string]string)
	slugCacheMu sync.RWMutex
)

// SlugNormalizer detects common dynamic path segments (UUIDs, numeric IDs,
// hex hashes, ISO dates) and replaces them with placeholder tags.
//
//	/users/abc123-def-456  ->  /users/:slug
//	/users/42              ->  /users/:id
//	/stats/2024-01-15       ->  /stats/:date
//	/files/a1b2c3d4e5f6    ->  /files/:hex
//
// Results are cached in a bounded map (10k entries) to prevent unbounded
// memory growth from high-cardinality paths.
func SlugNormalizer(path string) string {
	if path == "" {
		return path
	}

	slugCacheMu.RLock()
	cached, ok := slugCache[path]
	slugCacheMu.RUnlock()
	if ok {
		return cached
	}

	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		cached = "/"
		storeSlug(path, cached)
		return cached
	}

	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	normalized := make([]string, 0, len(segments))

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		switch {
		case uuidRegex.MatchString(seg):
			normalized = append(normalized, ":uuid")
		case numericRegex.MatchString(seg):
			normalized = append(normalized, ":id")
		case hexHashRegex.MatchString(seg):
			normalized = append(normalized, ":hex")
		case dateRegex.MatchString(seg):
			normalized = append(normalized, ":date")
		case slugRegex.MatchString(seg):
			normalized = append(normalized, ":slug")
		default:
			normalized = append(normalized, seg)
		}
	}

	result := "/" + strings.Join(normalized, "/")
	storeSlug(path, result)
	return result
}

func storeSlug(path, result string) {
	slugCacheMu.Lock()
	if len(slugCache) >= slugCacheCap {
		// evict all — simple, O(1) amortized, prevents slow leak
		slugCache = make(map[string]string, slugCacheCap)
	}
	slugCache[path] = result
	slugCacheMu.Unlock()
}

var (
	uuidRegex    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	numericRegex = regexp.MustCompile(`^\d+$`)
	hexHashRegex = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
	dateRegex    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	slugRegex    = regexp.MustCompile(`^[a-zA-Z0-9]+[-_][a-zA-Z0-9_-]{6,}$`)
)
