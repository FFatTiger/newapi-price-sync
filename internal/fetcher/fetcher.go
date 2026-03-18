package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"newapi-price-sync/internal/config"
	"newapi-price-sync/internal/models"
	"newapi-price-sync/pkg/normalize"
)

type Fetcher interface {
	Fetch(ctx context.Context) (models.PriceFields, error)
	Name() string
}

type HTTPFetcher struct {
	source     config.SourceConfig
	exchange   float64
	multiplier float64
	client     *http.Client
}

func New(source config.SourceConfig, exchangeRate, priceMultiplier float64) (Fetcher, error) {
	if !source.Enabled {
		return nil, fmt.Errorf("source %s disabled", source.Type)
	}
	f := &HTTPFetcher{
		source:     source,
		exchange:   exchangeRate,
		multiplier: priceMultiplier,
		client:     &http.Client{Timeout: source.Timeout},
	}
	return f, nil
}

func (f *HTTPFetcher) Name() string { return f.source.Type }

func (f *HTTPFetcher) Fetch(ctx context.Context) (models.PriceFields, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.source.URL, nil)
	if err != nil {
		return models.PriceFields{}, err
	}
	for k, v := range f.source.Headers {
		req.Header.Set(k, v)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return models.PriceFields{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.PriceFields{}, fmt.Errorf("http %s", resp.Status)
	}

	switch f.source.Type {
	case "models_dev":
		return f.fetchModelsDev(resp)
	case "openrouter":
		return f.fetchOpenRouter(resp)
	case "newapi_ratio":
		return f.fetchNewAPIRatio(resp)
	case "newapi_pricing":
		return f.fetchNewAPIPricing(resp)
	default:
		return models.PriceFields{}, fmt.Errorf("unsupported source type: %s", f.source.Type)
	}
}

type modelsDevProvider struct {
	Models map[string]struct {
		Cost struct {
			Input      *float64 `json:"input"`
			Output     *float64 `json:"output"`
			CacheRead  *float64 `json:"cache_read"`
			CacheWrite *float64 `json:"cache_write"`
		} `json:"cost"`
	} `json:"models"`
}

func (f *HTTPFetcher) fetchModelsDev(resp *http.Response) (models.PriceFields, error) {
	var raw map[string]modelsDevProvider
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return models.PriceFields{}, err
	}
	selected := map[string]struct {
		input      float64
		output     *float64
		cacheRead  *float64
		cacheWrite *float64
		provider   string
	}{}
	providers := make([]string, 0, len(raw))
	for p := range raw {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		for modelName, model := range raw[provider].Models {
			if model.Cost.Input == nil || *model.Cost.Input < 0 {
				continue
			}
			cur, ok := selected[modelName]
			candidateInput := *model.Cost.Input
			if !ok || betterCandidate(cur.input, candidateInput) {
				selected[modelName] = struct {
					input      float64
					output     *float64
					cacheRead  *float64
					cacheWrite *float64
					provider   string
				}{
					input:      candidateInput,
					output:     model.Cost.Output,
					cacheRead:  model.Cost.CacheRead,
					cacheWrite: model.Cost.CacheWrite,
					provider:   provider,
				}
			}
		}
	}
	out := models.NewPriceFields()
	for modelName, item := range selected {
		out.ModelRatio[modelName] = normalize.ModelRatioFromUSDPer1M(item.input, f.exchange, f.multiplier)
		if item.output != nil {
			if ratio, ok := normalize.Ratio(*item.output, item.input); ok {
				out.CompletionRatio[modelName] = ratio
			}
		}
		if item.cacheRead != nil {
			if ratio, ok := normalize.Ratio(*item.cacheRead, item.input); ok {
				out.CacheRatio[modelName] = ratio
			}
		}
		if item.cacheWrite != nil {
			if ratio, ok := normalize.Ratio(*item.cacheWrite, item.input); ok {
				out.CreateCacheRatio[modelName] = ratio
			}
		}
	}
	return out, nil
}

func betterCandidate(current, next float64) bool {
	if current == 0 {
		return next > 0
	}
	if next == 0 {
		return false
	}
	return next < current
}

func (f *HTTPFetcher) fetchOpenRouter(resp *http.Response) (models.PriceFields, error) {
	var raw struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt          string `json:"prompt"`
				Completion      string `json:"completion"`
				InputCacheRead  string `json:"input_cache_read"`
				InputCacheWrite string `json:"input_cache_write"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return models.PriceFields{}, err
	}
	out := models.NewPriceFields()
	for _, m := range raw.Data {
		prompt, err1 := parseStringFloat(m.Pricing.Prompt)
		completion, err2 := parseStringFloat(m.Pricing.Completion)
		if err1 != nil || prompt < 0 {
			continue
		}
		if err2 != nil || completion < 0 {
			completion = 0
		}
		promptPer1M := prompt * 1_000_000
		completionPer1M := completion * 1_000_000
		out.ModelRatio[m.ID] = normalize.ModelRatioFromUSDPer1M(promptPer1M, f.exchange, f.multiplier)
		if ratio, ok := normalize.Ratio(completionPer1M, promptPer1M); ok {
			out.CompletionRatio[m.ID] = ratio
		}
		if s := strings.TrimSpace(m.Pricing.InputCacheRead); s != "" {
			if cacheRead, err := parseStringFloat(s); err == nil && cacheRead >= 0 {
				if ratio, ok := normalize.Ratio(cacheRead*1_000_000, promptPer1M); ok {
					out.CacheRatio[m.ID] = ratio
				}
			}
		}
		if s := strings.TrimSpace(m.Pricing.InputCacheWrite); s != "" {
			if cacheWrite, err := parseStringFloat(s); err == nil && cacheWrite >= 0 {
				if ratio, ok := normalize.Ratio(cacheWrite*1_000_000, promptPer1M); ok {
					out.CreateCacheRatio[m.ID] = ratio
				}
			}
		}
	}
	return out, nil
}

func (f *HTTPFetcher) fetchNewAPIRatio(resp *http.Response) (models.PriceFields, error) {
	var raw struct {
		Success bool               `json:"success"`
		Data    models.PriceFields `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return models.PriceFields{}, err
	}
	if !raw.Success {
		return models.PriceFields{}, fmt.Errorf("newapi ratio response success=false")
	}
	return raw.Data, nil
}

func (f *HTTPFetcher) fetchNewAPIPricing(resp *http.Response) (models.PriceFields, error) {
	var raw struct {
		Success bool `json:"success"`
		Data    []struct {
			ModelName        string   `json:"model_name"`
			QuotaType        int      `json:"quota_type"`
			ModelRatio       float64  `json:"model_ratio"`
			ModelPrice       float64  `json:"model_price"`
			CompletionRatio  float64  `json:"completion_ratio"`
			CacheRatio       *float64 `json:"cache_ratio"`
			CreateCacheRatio *float64 `json:"create_cache_ratio"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return models.PriceFields{}, err
	}
	if !raw.Success {
		return models.PriceFields{}, fmt.Errorf("newapi pricing response success=false")
	}
	out := models.NewPriceFields()
	for _, item := range raw.Data {
		if item.QuotaType == 1 {
			out.ModelPrice[item.ModelName] = normalize.ModelPriceFromUnitPrice(item.ModelPrice, f.exchange, f.multiplier)
		} else {
			// ratio-like fields already normalized for NewAPI, just scale monetary base fields when source is pricing.
			out.ModelRatio[item.ModelName] = normalize.Round6(item.ModelRatio * f.exchange * f.multiplier)
			out.CompletionRatio[item.ModelName] = normalize.Round6(item.CompletionRatio)
			if item.CacheRatio != nil {
				out.CacheRatio[item.ModelName] = normalize.Round6(*item.CacheRatio)
			}
			if item.CreateCacheRatio != nil {
				out.CreateCacheRatio[item.ModelName] = normalize.Round6(*item.CreateCacheRatio)
			}
		}
	}
	return out, nil
}

func parseStringFloat(v string) (float64, error) {
	var out float64
	_, err := fmt.Sscanf(v, "%f", &out)
	return out, err
}
