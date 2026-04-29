package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lumiforge/wb_api_agent_system/internal/domain/wbregistry"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ wbregistry.OperationEmbeddingStore = (*WBRegistryEmbeddingStore)(nil)

// PURPOSE: Persists registry operation embeddings in a dedicated SQLite database.
type WBRegistryEmbeddingStore struct {
	db *gorm.DB
}

type wbRegistryEmbeddingRecord struct {
	OperationID string `gorm:"primaryKey;not null"`
	Model       string `gorm:"primaryKey;not null"`
	Dimensions  int    `gorm:"primaryKey;not null"`

	SourceFile  string `gorm:"not null"`
	ContentHash string `gorm:"not null"`
	VectorJSON  string `gorm:"not null"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (wbRegistryEmbeddingRecord) TableName() string {
	return "wb_registry_operation_embeddings"
}

func NewWBRegistryEmbeddingStore(db *gorm.DB) *WBRegistryEmbeddingStore {
	return &WBRegistryEmbeddingStore{db: db}
}

func AutoMigrateWBRegistryEmbeddingStore(db *gorm.DB) error {
	if err := db.AutoMigrate(&wbRegistryEmbeddingRecord{}); err != nil {
		return fmt.Errorf("auto migrate wb registry embedding store: %w", err)
	}

	return nil
}

func (s *WBRegistryEmbeddingStore) UpsertOperationEmbedding(
	ctx context.Context,
	embedding wbregistry.OperationEmbedding,
) error {
	if embedding.OperationID == "" {
		return fmt.Errorf("operation_id is required")
	}
	if embedding.Model == "" {
		return fmt.Errorf("model is required")
	}
	if embedding.Dimensions <= 0 {
		return fmt.Errorf("dimensions must be positive")
	}
	if len(embedding.Vector) != embedding.Dimensions {
		return fmt.Errorf("vector dimensions mismatch: dimensions=%d vector_len=%d", embedding.Dimensions, len(embedding.Vector))
	}
	if embedding.ContentHash == "" {
		return fmt.Errorf("content_hash is required")
	}

	vectorJSON, err := json.Marshal(embedding.Vector)
	if err != nil {
		return fmt.Errorf("marshal embedding vector: %w", err)
	}

	now := time.Now().UTC()
	record := wbRegistryEmbeddingRecord{
		OperationID: embedding.OperationID,
		Model:       embedding.Model,
		Dimensions:  embedding.Dimensions,
		SourceFile:  embedding.SourceFile,
		ContentHash: embedding.ContentHash,
		VectorJSON:  string(vectorJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "operation_id"},
			{Name: "model"},
			{Name: "dimensions"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_file",
			"content_hash",
			"vector_json",
			"updated_at",
		}),
	}).Create(&record).Error
	if err != nil {
		return fmt.Errorf("upsert operation embedding: %w", err)
	}

	return nil
}

func (s *WBRegistryEmbeddingStore) GetOperationEmbedding(
	ctx context.Context,
	operationID string,
	model string,
	dimensions int,
) (*wbregistry.OperationEmbedding, error) {
	var record wbRegistryEmbeddingRecord

	err := s.db.WithContext(ctx).
		Where("operation_id = ? AND model = ? AND dimensions = ?", operationID, model, dimensions).
		Limit(1).
		Find(&record).
		Error
	if err != nil {
		return nil, fmt.Errorf("get operation embedding: %w", err)
	}

	if record.OperationID == "" {
		return nil, nil
	}

	embedding, err := operationEmbeddingRecordToEntity(record)
	if err != nil {
		return nil, err
	}

	return &embedding, nil
}

func (s *WBRegistryEmbeddingStore) ListOperationEmbeddings(
	ctx context.Context,
	model string,
	dimensions int,
) ([]wbregistry.OperationEmbedding, error) {
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if dimensions <= 0 {
		return nil, fmt.Errorf("dimensions must be positive")
	}

	var records []wbRegistryEmbeddingRecord

	if err := s.db.WithContext(ctx).
		Where("model = ? AND dimensions = ?", model, dimensions).
		Order("source_file ASC, operation_id ASC").
		Find(&records).
		Error; err != nil {
		return nil, fmt.Errorf("list operation embeddings: %w", err)
	}

	embeddings := make([]wbregistry.OperationEmbedding, 0, len(records))
	for _, record := range records {
		embedding, err := operationEmbeddingRecordToEntity(record)
		if err != nil {
			return nil, err
		}

		embeddings = append(embeddings, embedding)
	}

	return embeddings, nil
}

func operationEmbeddingRecordToEntity(record wbRegistryEmbeddingRecord) (wbregistry.OperationEmbedding, error) {
	var vector []float64
	if err := json.Unmarshal([]byte(record.VectorJSON), &vector); err != nil {
		return wbregistry.OperationEmbedding{}, fmt.Errorf("unmarshal operation embedding vector: %w", err)
	}

	return wbregistry.OperationEmbedding{
		OperationID: record.OperationID,
		SourceFile:  record.SourceFile,
		Model:       record.Model,
		Dimensions:  record.Dimensions,
		ContentHash: record.ContentHash,
		Vector:      vector,
	}, nil
}

func (s *WBRegistryEmbeddingStore) StatsOperationEmbeddings(
	ctx context.Context,
	model string,
	dimensions int,
) (wbregistry.OperationEmbeddingStats, error) {
	if model == "" {
		return wbregistry.OperationEmbeddingStats{}, fmt.Errorf("model is required")
	}
	if dimensions <= 0 {
		return wbregistry.OperationEmbeddingStats{}, fmt.Errorf("dimensions must be positive")
	}

	var total int64

	if err := s.db.WithContext(ctx).
		Model(&wbRegistryEmbeddingRecord{}).
		Where("model = ? AND dimensions = ?", model, dimensions).
		Count(&total).
		Error; err != nil {
		return wbregistry.OperationEmbeddingStats{}, fmt.Errorf("count operation embeddings: %w", err)
	}

	return wbregistry.OperationEmbeddingStats{
		Total: total,
	}, nil
}
