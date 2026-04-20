package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// diffContextLines is the number of unchanged lines shown around each change,
// matching the standard diff -u default.
const diffContextLines = 3

type configDiffReader interface {
	GetAtRevision(ctx context.Context, path, namespace string, revision int64) (*domain.HistoryEntry, error)
}

type DiffUseCase struct {
	configs configDiffReader
}

func NewDiffUseCase(configs configDiffReader) *DiffUseCase {
	return &DiffUseCase{configs: configs}
}

func (uc *DiffUseCase) GetDiff(
	ctx context.Context,
	path, namespace string,
	fromRevision, toRevision int64,
) (*domain.ConfigDiff, error) {
	if err := uc.validate(path, namespace, fromRevision, toRevision); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	toEntry, err := uc.configs.GetAtRevision(ctx, path, namespace, toRevision)
	if err != nil {
		return nil, fmt.Errorf("get revision %d: %w", toRevision, err)
	}

	var fromContent string

	var actualFromRevision int64

	if fromRevision > 0 {
		fromEntry, err := uc.configs.GetAtRevision(ctx, path, namespace, fromRevision)
		if err != nil {
			return nil, fmt.Errorf("get revision %d: %w", fromRevision, err)
		}

		fromContent, err = normalizeContent(path, fromEntry.Content)
		if err != nil {
			return nil, fmt.Errorf("normalize revision %d: %w", fromRevision, err)
		}

		actualFromRevision = fromEntry.Revision
	}

	toContent, err := normalizeContent(path, toEntry.Content)
	if err != nil {
		return nil, fmt.Errorf("normalize revision %d: %w", toRevision, err)
	}

	unifiedDiff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(fromContent),
		B:        difflib.SplitLines(toContent),
		FromFile: fmt.Sprintf("revision %d", actualFromRevision),
		ToFile:   fmt.Sprintf("revision %d", toEntry.Revision),
		Context:  diffContextLines,
	})
	if err != nil {
		return nil, fmt.Errorf("compute diff: %w", err)
	}

	return &domain.ConfigDiff{
		FromRevision: actualFromRevision,
		ToRevision:   toEntry.Revision,
		FromContent:  strings.TrimRight(fromContent, "\n"),
		ToContent:    strings.TrimRight(toContent, "\n"),
		Diff:         unifiedDiff,
	}, nil
}

func (uc *DiffUseCase) validate(path, namespace string, fromRevision, toRevision int64) error {
	if path == "" {
		return domain.NewValidationError("path", "required")
	}

	if namespace == "" {
		return domain.NewValidationError("namespace", "required")
	}

	if toRevision == 0 {
		return domain.NewValidationError("to_revision", "must be greater than 0")
	}

	if fromRevision > toRevision {
		return domain.NewValidationError("from_revision", "must not be greater than to_revision")
	}

	return nil
}

func normalizeContent(path, content string) (string, error) {
	if content == "" {
		return "", nil
	}

	normalized, err := domain.NormalizeContent(content, domain.DetectFormatFromPath(path))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidContent) {
			return content, nil
		}

		return "", fmt.Errorf("normalize content: %w", err)
	}

	return normalized, nil
}
