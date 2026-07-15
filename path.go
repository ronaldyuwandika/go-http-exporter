package httpexporter

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

const slugCacheCap = 10_000

var (
	slugCache sync.Map
	slugCount atomic.Int64
)

// SlugNormalizer detects common dynamic path segments (UUIDs, numeric IDs,
// hex hashes, ISO dates) and replaces them with placeholder tags.
//
//	/users/abc123-def-456  ->  /users/:slug
//	/users/42              ->  /users/:id
//	/stats/2024-01-15       ->  /stats/:date
//	/files/a1b2c3d4e5f6    ->  /files/:hex
//
// Cache uses sync.Map for lock-free reads at high RPS. Bounded at ~10k
// entries; when exceeded the entire cache is cleared to prevent memory leaks.
func SlugNormalizer(path string) string {
	if path == "" {
		return path
	}

	if cached, ok := slugCache.Load(path); ok {
		return cached.(string)
	}

	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		slugStore(path, "/")
		return "/"
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
	slugStore(path, result)
	return result
}

func slugStore(path, result string) {
	slugCache.Store(path, result)
	if slugCount.Add(1) > slugCacheCap {
		slugCache.Clear()
		slugCount.Store(0)
	}
}

var (
	uuidRegex    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	numericRegex = regexp.MustCompile(`^\d+$`)
	hexHashRegex = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
	dateRegex    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	slugRegex    = regexp.MustCompile(`^[a-zA-Z0-9]+[-_][a-zA-Z0-9_-]{6,}$`)
)
