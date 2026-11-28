package valuation

import (
	"context"
	"sync"
	"time"
)

type memoryDraft struct {
	value     string
	expiresAt time.Time
}

type memDraftRepository struct {
	mu   sync.Mutex
	rows map[string]memoryDraft
}

func newMemDraftRepository() *memDraftRepository {
	return &memDraftRepository{rows: map[string]memoryDraft{}}
}

func (m *memDraftRepository) CreateDraft(_ context.Context, draft *storedDraft, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := draftKey(draft.Draft.ID)
	if _, exists := m.rows[key]; exists {
		return ErrDraftVersionConflict
	}
	value, err := encodeStoredDraft(draft)
	if err != nil {
		return err
	}
	m.rows[key] = memoryDraft{value: value, expiresAt: expiresAt}
	return nil
}

func (m *memDraftRepository) GetDraft(_ context.Context, id string, now time.Time) (*storedDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.rows[draftKey(id)]
	if !ok || !row.expiresAt.After(now) {
		return nil, ErrDraftNotFound
	}
	return decodeStoredDraft(row.value)
}

func (m *memDraftRepository) SaveDraft(_ context.Context, id, expected string, draft *storedDraft, expiresAt, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := draftKey(id)
	row, ok := m.rows[key]
	if !ok || !row.expiresAt.After(now) || row.value != expected {
		return false, nil
	}
	value, err := encodeStoredDraft(draft)
	if err != nil {
		return false, err
	}
	m.rows[key] = memoryDraft{value: value, expiresAt: expiresAt}
	return true, nil
}

func (m *memDraftRepository) DeleteDraft(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, draftKey(id))
	return nil
}

type memOperationRepository struct {
	mu          sync.Mutex
	operations  map[string]*ConversationOperation
	evaluations OperationEvaluationWriter
}

func newMemOperationRepository(evaluations OperationEvaluationWriter) *memOperationRepository {
	return &memOperationRepository{operations: map[string]*ConversationOperation{}, evaluations: evaluations}
}

func (m *memOperationRepository) GetOperation(_ context.Context, operationID string) (*ConversationOperation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	operation, ok := m.operations[operationID]
	if !ok {
		return nil, ErrOperationNotFound
	}
	copied := *operation
	return &copied, nil
}

// cloneOperationRecord 返回与内部存储互不共享的记录副本：后台 goroutine 会在锁内改写
// 状态/结果，响应序列化与读取都必须拿独立对象，否则与 handler 的 JSON 编码并发读写同一指针。
func cloneOperationRecord(operation *ConversationOperation) *ConversationOperation {
	copied := *operation
	return &copied
}

func (m *memOperationRepository) AcceptOperation(_ context.Context, operation *ConversationOperation,
	persist func(OperationEvaluationWriter) (int64, string, error)) (*ConversationOperation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.operations[operation.OperationID]; ok {
		return cloneOperationRecord(existing), false, nil
	}
	evaluationID, failureCode, err := persist(m.evaluations)
	if err != nil && failureCode == "" {
		return nil, false, err
	}
	stored := *operation
	stored.ID = stored.OperationID
	stored.Status = OperationStatusSucceeded
	if err != nil {
		stored.Status = OperationStatusFailed
		stored.ErrorCode = failureCode
	}
	stored.EvaluationID = evaluationID
	stored.UpdatedAt = time.Now()
	m.operations[stored.OperationID] = &stored
	return cloneOperationRecord(&stored), true, nil
}
