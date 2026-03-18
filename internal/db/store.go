package db

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"newapi-price-sync/internal/config"
	"newapi-price-sync/internal/models"
)

type Store struct {
	db *gorm.DB
}

func Open(cfg config.DatabaseConfig) (*Store, error) {
	var (
		db  *gorm.DB
		err error
	)
	switch cfg.Type {
	case "postgres":
		db, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	case "mysql":
		dsn := cfg.DSN
		if !strings.Contains(dsn, "parseTime=") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		db, err = gorm.Open(sqlite.Open(cfg.SQLitePath), &gorm.Config{})
	}
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Store) LoadCurrent() (models.PriceFields, error) {
	keys := []string{"ModelRatio", "CompletionRatio", "CacheRatio", "CreateCacheRatio", "ModelPrice"}
	var opts []models.Option
	if err := s.db.Where("key IN ?", keys).Find(&opts).Error; err != nil {
		return models.PriceFields{}, fmt.Errorf("load options: %w", err)
	}
	out := models.NewPriceFields()
	for _, opt := range opts {
		target := pickMap(out, opt.Key)
		if target == nil || strings.TrimSpace(opt.Value) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(opt.Value), target); err != nil {
			return models.PriceFields{}, fmt.Errorf("decode %s: %w", opt.Key, err)
		}
	}
	return out, nil
}

func (s *Store) Upsert(fields models.PriceFields) error {
	payloads, err := marshalFields(fields)
	if err != nil {
		return err
	}
	for _, opt := range payloads {
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&opt).Error; err != nil {
			return fmt.Errorf("upsert %s: %w", opt.Key, err)
		}
	}
	return nil
}

func marshalFields(fields models.PriceFields) ([]models.Option, error) {
	items := []struct {
		key string
		m   map[string]float64
	}{
		{"ModelRatio", fields.ModelRatio},
		{"CompletionRatio", fields.CompletionRatio},
		{"CacheRatio", fields.CacheRatio},
		{"CreateCacheRatio", fields.CreateCacheRatio},
		{"ModelPrice", fields.ModelPrice},
	}
	out := make([]models.Option, 0, len(items))
	for _, item := range items {
		b, err := json.Marshal(item.m)
		if err != nil {
			return nil, fmt.Errorf("marshal %s: %w", item.key, err)
		}
		out = append(out, models.Option{Key: item.key, Value: string(b)})
	}
	return out, nil
}

func pickMap(fields models.PriceFields, key string) *map[string]float64 {
	switch key {
	case "ModelRatio":
		return &fields.ModelRatio
	case "CompletionRatio":
		return &fields.CompletionRatio
	case "CacheRatio":
		return &fields.CacheRatio
	case "CreateCacheRatio":
		return &fields.CreateCacheRatio
	case "ModelPrice":
		return &fields.ModelPrice
	default:
		return nil
	}
}
