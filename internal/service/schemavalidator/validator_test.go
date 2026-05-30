package schemavalidator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/schemavalidator"
	schemavalidatormock "github.com/sergeyslonimsky/elara/internal/service/schemavalidator/mocks"
)

const (
	testJSONSchema   = `{"type": "object", "required": ["host"], "properties": {"host": {"type": "string"}}}`
	validJSONContent = `{"host": "localhost"}`
	invalidContent   = `{"port": 5432}`
)

func TestValidator_Validate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exactSchema := `{"type": "object", "properties": {"port": {"type": "integer"}}}`

	type input struct {
		namespace string
		path      string
		content   string
		format    domain.Format
	}

	tests := []struct {
		name          string
		input         input
		mockFunc      func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator
		errIs         error
		wantErr       string
		wantSchemaErr bool
	}{
		{
			name: "NoSchemas",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   validJSONContent,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(nil, nil)

				return schemavalidator.New(repo)
			},
		},
		{
			name: "PatternMiss",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   validJSONContent,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/other/**",
						JSONSchema:  testJSONSchema,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
		},
		{
			name: "PatternMatch_ValidContent",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   validJSONContent,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/app/**",
						JSONSchema:  testJSONSchema,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
		},
		{
			name: "PatternMatch_InvalidContent",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   invalidContent,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/app/**",
						JSONSchema:  testJSONSchema,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
			wantSchemaErr: true,
		},
		{
			name: "YAML_Valid",
			input: input{
				namespace: "ns",
				path:      "/app/config.yaml",
				content:   "host: localhost\n",
				format:    domain.FormatYAML,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/app/**",
						JSONSchema:  testJSONSchema,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
		},
		{
			name: "FormatOther_Skip",
			input: input{
				namespace: "ns",
				path:      "/app/config.txt",
				content:   "any content",
				format:    domain.FormatOther,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				repo := schemavalidatormock.NewMockstorage(ctrl)

				return schemavalidator.New(repo)
			},
		},
		{
			name: "MostSpecificPatternWins",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   invalidContent,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/app/**",
						JSONSchema:  testJSONSchema,
						CreatedAt:   now,
					},
					{
						ID:          "2",
						Namespace:   "ns",
						PathPattern: "/app/config.json",
						JSONSchema:  exactSchema,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
		},
		{
			name: "StorageListError",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   validJSONContent,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(nil, errors.New("db error"))

				return schemavalidator.New(repo)
			},
			wantErr: "list schemas: db error",
		},
		{
			name: "ToJSONValueError",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   `{invalid json`,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/app/**",
						JSONSchema:  testJSONSchema,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
			wantErr: "convert content to json: unmarshal json:",
		},
		{
			name: "CompileSchemaError",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   validJSONContent,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/app/**",
						JSONSchema:  `{broken schema`,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
			wantErr: "compile schema: unmarshal schema json:",
		},
		{
			name: "ValidateError_NestedViolations",
			input: input{
				namespace: "ns",
				path:      "/app/config.json",
				content:   `{"host": 123}`,
				format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) *schemavalidator.Validator {
				schemas := []*domain.SchemaAttachment{
					{
						ID:          "1",
						Namespace:   "ns",
						PathPattern: "/app/**",
						JSONSchema:  `{"type": "object", "properties": {"host": {"type": "string"}}}`,
						CreatedAt:   now,
					},
				}
				repo := schemavalidatormock.NewMockstorage(ctrl)
				repo.EXPECT().List(ctx, "ns").Return(schemas, nil)

				return schemavalidator.New(repo)
			},
			wantSchemaErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(t.Context(), ctrl)

			err := sut.Validate(
				t.Context(),
				tt.input.namespace,
				tt.input.path,
				tt.input.content,
				tt.input.format,
			)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantSchemaErr {
				var sve *domain.SchemaValidationError
				require.ErrorAs(
					t,
					err,
					&sve,
					"expected SchemaValidationError, got %T: %v",
					err,
					err,
				)
				assert.NotEmpty(t, sve.Violations)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
