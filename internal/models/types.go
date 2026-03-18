package models

type Option struct {
	Key   string `gorm:"primaryKey;column:key"`
	Value string `gorm:"column:value"`
}

func (Option) TableName() string {
	return "options"
}

type PriceFields struct {
	ModelRatio       map[string]float64 `json:"model_ratio"`
	CompletionRatio  map[string]float64 `json:"completion_ratio"`
	CacheRatio       map[string]float64 `json:"cache_ratio"`
	CreateCacheRatio map[string]float64 `json:"create_cache_ratio"`
	ModelPrice       map[string]float64 `json:"model_price"`
}

func NewPriceFields() PriceFields {
	return PriceFields{
		ModelRatio:       map[string]float64{},
		CompletionRatio:  map[string]float64{},
		CacheRatio:       map[string]float64{},
		CreateCacheRatio: map[string]float64{},
		ModelPrice:       map[string]float64{},
	}
}
