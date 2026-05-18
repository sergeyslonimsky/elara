package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

const diffContextLines = 3

type DiffInput struct {
	Namespace string
	Path      string
	V1        int64
	V2        int64
}

func (s *Service) Diff(ctx context.Context, in DiffInput) (*domain.ConfigDiff, error) {
	if err := s.validateDiff(in.Path, in.Namespace, in.V1, in.V2); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	if err := auth.CheckAccess(ctx, s.enforcer, in.Namespace, domain.ObjectConfig, domain.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	toEntry, err := s.storage.GetAtRevision(ctx, in.Path, in.Namespace, in.V2)
	if err != nil {
		return nil, fmt.Errorf("get revision %d: %w", in.V2, err)
	}

	var fromContent string

	var actualFromRevision int64

	if in.V1 > 0 {
		fromEntry, err := s.storage.GetAtRevision(ctx, in.Path, in.Namespace, in.V1)
		if err != nil {
			return nil, fmt.Errorf("get revision %d: %w", in.V1, err)
		}

		fromContent, err = normalizeDiffContent(in.Path, fromEntry.Content)
		if err != nil {
			return nil, fmt.Errorf("normalize revision %d: %w", in.V1, err)
		}

		actualFromRevision = fromEntry.Revision
	}

	toContent, err := normalizeDiffContent(in.Path, toEntry.Content)
	if err != nil {
		return nil, fmt.Errorf("normalize revision %d: %w", in.V2, err)
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

func (s *Service) validateDiff(path, namespace string, fromRevision, toRevision int64) error {
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

func normalizeDiffContent(path, content string) (string, error) {
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
