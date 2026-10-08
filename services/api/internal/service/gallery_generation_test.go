package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestGalleryGenerationTaskInsertErrorHidesActiveSpecConstraint(t *testing.T) {
	databaseErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "uq_gallery_generation_active_spec",
		Message:        "duplicate key value violates unique constraint",
	}

	err := galleryGenerationTaskInsertError(databaseErr)
	if !errors.Is(err, ErrGalleryGenerationSpecActive) {
		t.Fatalf("expected active-spec conflict, got %v", err)
	}
	if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
		t.Fatalf("database internals leaked to administrator: %v", err)
	}
}

func TestGalleryGenerationTaskInsertErrorPreservesUnexpectedFailure(t *testing.T) {
	databaseErr := errors.New("connection closed")
	err := galleryGenerationTaskInsertError(databaseErr)
	if errors.Is(err, ErrGalleryGenerationSpecActive) {
		t.Fatalf("unexpected failure classified as active-spec conflict: %v", err)
	}
	if !errors.Is(err, databaseErr) || !strings.Contains(err.Error(), "创建生成任务失败") {
		t.Fatalf("unexpected failure was not preserved: %v", err)
	}
}
