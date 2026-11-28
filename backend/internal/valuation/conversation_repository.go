package valuation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DraftRepository interface {
	CreateDraft(ctx context.Context, draft *storedDraft, expiresAt time.Time) error
	GetDraft(ctx context.Context, id string, now time.Time) (*storedDraft, error)
	SaveDraft(ctx context.Context, id, expected string, draft *storedDraft, expiresAt, now time.Time) (bool, error)
	DeleteDraft(ctx context.Context, id string) error
}

type draftRepository struct {
	pool *pgxpool.Pool
}

func NewDraftRepository(pool *pgxpool.Pool) DraftRepository {
	return &draftRepository{pool: pool}
}

func encodeStoredDraft(draft *storedDraft) (string, error) {
	value, err := json.Marshal(draft)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (r *draftRepository) CreateDraft(ctx context.Context, draft *storedDraft, expiresAt time.Time) error {
	value, err := encodeStoredDraft(draft)
	if err != nil {
		return fmt.Errorf("marshal valuation draft: %w", err)
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO ephemeral_kv (cache_key, cache_value, expires_at) VALUES ($1, $2, $3)`,
		draftKey(draft.Draft.ID), string(value), expiresAt)
	return err
}

func (r *draftRepository) GetDraft(ctx context.Context, id string, now time.Time) (*storedDraft, error) {
	var value string
	err := r.pool.QueryRow(ctx, `SELECT cache_value FROM ephemeral_kv WHERE cache_key = $1 AND expires_at > $2`,
		draftKey(id), now).Scan(&value)
	if err != nil {
		if pgx.ErrNoRows == err {
			return nil, ErrDraftNotFound
		}
		return nil, fmt.Errorf("load valuation draft: %w", err)
	}
	return decodeStoredDraft(value)
}

func (r *draftRepository) SaveDraft(ctx context.Context, id, expected string, draft *storedDraft, expiresAt, now time.Time) (bool, error) {
	value, err := json.Marshal(draft)
	if err != nil {
		return false, fmt.Errorf("marshal valuation draft: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `UPDATE ephemeral_kv SET cache_value = $2, expires_at = $3
		WHERE cache_key = $1 AND cache_value = $4 AND expires_at > $5`,
		draftKey(id), string(value), expiresAt, expected, now)
	if err != nil {
		return false, fmt.Errorf("save valuation draft: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *draftRepository) DeleteDraft(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM ephemeral_kv WHERE cache_key = $1`, draftKey(id))
	return err
}

func draftKey(id string) string { return "valuation:draft:" + id }

func decodeStoredDraft(value string) (*storedDraft, error) {
	var draft storedDraft
	if err := json.Unmarshal([]byte(value), &draft); err != nil {
		return nil, fmt.Errorf("decode valuation draft: %w", err)
	}
	draft.Raw = value
	return &draft, nil
}
