package merger

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"newapi-price-sync/internal/models"
)

func Merge(current models.PriceFields, incoming []models.PriceFields, include, exclude []string, preserveUnmentioned bool) models.PriceFields {
	out := models.NewPriceFields()
	if preserveUnmentioned {
		out = cloneFields(current)
	}
	mergeMap(out.ModelRatio, collectMaps(incoming, func(f models.PriceFields) map[string]float64 { return f.ModelRatio }), include, exclude)
	mergeMap(out.CompletionRatio, collectMaps(incoming, func(f models.PriceFields) map[string]float64 { return f.CompletionRatio }), include, exclude)
	mergeMap(out.CacheRatio, collectMaps(incoming, func(f models.PriceFields) map[string]float64 { return f.CacheRatio }), include, exclude)
	mergeMap(out.CreateCacheRatio, collectMaps(incoming, func(f models.PriceFields) map[string]float64 { return f.CreateCacheRatio }), include, exclude)
	mergeMap(out.ModelPrice, collectMaps(incoming, func(f models.PriceFields) map[string]float64 { return f.ModelPrice }), include, exclude)
	return out
}

func collectMaps(incoming []models.PriceFields, pick func(models.PriceFields) map[string]float64) []map[string]float64 {
	out := make([]map[string]float64, 0, len(incoming))
	for _, fields := range incoming {
		out = append(out, pick(fields))
	}
	return out
}

func mergeMap(dst map[string]float64, srcMaps []map[string]float64, include, exclude []string) {
	resolver := newAliasResolver(dst)
	for _, src := range srcMaps {
		for k, v := range src {
			if !allowed(k, include, exclude) {
				continue
			}
			matched := resolver.resolve(k)
			if len(matched) == 0 {
				dst[k] = v
				resolver.add(k)
				continue
			}
			for _, key := range matched {
				dst[key] = v
			}
		}
	}
}

func allowed(name string, include, exclude []string) bool {
	if len(include) > 0 {
		ok := false
		for _, p := range include {
			if regexp.MustCompile(p).MatchString(name) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, p := range exclude {
		if regexp.MustCompile(p).MatchString(name) {
			return false
		}
	}
	return true
}

type aliasResolver struct {
	aliasToKeys map[string]map[string]struct{}
}

func newAliasResolver(existing map[string]float64) *aliasResolver {
	r := &aliasResolver{aliasToKeys: map[string]map[string]struct{}{}}
	for k := range existing {
		r.add(k)
	}
	return r
}

func (r *aliasResolver) add(key string) {
	for _, alias := range aliasesFor(key) {
		if alias == "" {
			continue
		}
		if r.aliasToKeys[alias] == nil {
			r.aliasToKeys[alias] = map[string]struct{}{}
		}
		r.aliasToKeys[alias][key] = struct{}{}
	}
}

func (r *aliasResolver) resolve(name string) []string {
	seen := map[string]struct{}{}
	for _, alias := range aliasesFor(name) {
		for key := range r.aliasToKeys[alias] {
			seen[key] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func aliasesFor(name string) []string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return nil
	}
	parts := []string{trimmed}
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 && idx+1 < len(trimmed) {
		parts = append(parts, trimmed[idx+1:])
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		for _, alias := range []string{part, squashModelName(part)} {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			if _, ok := seen[alias]; ok {
				continue
			}
			seen[alias] = struct{}{}
			out = append(out, alias)
		}
	}
	return out
}

func squashModelName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func cloneMap(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneFields(src models.PriceFields) models.PriceFields {
	return models.PriceFields{
		ModelRatio:       cloneMap(src.ModelRatio),
		CompletionRatio:  cloneMap(src.CompletionRatio),
		CacheRatio:       cloneMap(src.CacheRatio),
		CreateCacheRatio: cloneMap(src.CreateCacheRatio),
		ModelPrice:       cloneMap(src.ModelPrice),
	}
}
