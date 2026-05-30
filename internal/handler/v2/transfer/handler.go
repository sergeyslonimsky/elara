package transfer

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=transfer_mock -source=handler.go

type (
	authz interface {
		Require(ctx context.Context, object domain.Object, action domain.Action, domainStr string) error
	}

	usecase interface {
		ExportNamespace(
			ctx context.Context,
			namespace string,
			asZip bool,
			enc transferv1.BundleEncoding,
		) ([]byte, string, string, error)
		ExportAll(
			ctx context.Context,
			asZip bool,
			enc transferv1.BundleEncoding,
			layout transferv1.ZipLayout,
		) ([]byte, string, string, error)
		Import(
			ctx context.Context,
			data []byte,
			onConflict transferv1.ConflictResolution,
			dryRun bool,
			targetNamespace string,
		) (*domain.ImportReport, error)
	}
)

type Handler struct {
	authz authz
	uc    usecase
}

func New(authz authz, uc usecase) *Handler {
	return &Handler{authz: authz, uc: uc}
}

func (h *Handler) ExportNamespace(
	ctx context.Context,
	req *connect.Request[transferv1.ExportNamespaceRequest],
) (*connect.Response[transferv1.ExportNamespaceResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectNamespace, domain.ActionRead, req.Msg.GetNamespace()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	data, ct, filename, err := h.uc.ExportNamespace(
		ctx,
		req.Msg.GetNamespace(),
		req.Msg.GetZip(),
		req.Msg.GetEncoding(),
	)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&transferv1.ExportNamespaceResponse{
		Data:        data,
		ContentType: ct,
		Filename:    filename,
	}), nil
}

// ExportAll is gate-less at the handler boundary; the use case filters
// namespaces per the caller's transfer:read permissions via the PDP. A caller
// with no read access on any namespace gets back an empty bundle rather than
// a 403.
func (h *Handler) ExportAll(
	ctx context.Context,
	req *connect.Request[transferv1.ExportAllRequest],
) (*connect.Response[transferv1.ExportAllResponse], error) {
	data, ct, filename, err := h.uc.ExportAll(
		ctx,
		req.Msg.GetZip(),
		req.Msg.GetEncoding(),
		req.Msg.GetZipLayout(),
	)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&transferv1.ExportAllResponse{
		Data:        data,
		ContentType: ct,
		Filename:    filename,
	}), nil
}

func (h *Handler) ImportNamespace(
	ctx context.Context,
	req *connect.Request[transferv1.ImportNamespaceRequest],
) (*connect.Response[transferv1.ImportNamespaceResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectNamespace, domain.ActionWrite, req.Msg.GetNamespace()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	report, err := h.uc.Import(
		ctx,
		req.Msg.GetData(),
		req.Msg.GetOnConflict(),
		req.Msg.GetDryRun(),
		req.Msg.GetNamespace(),
	)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := &transferv1.ImportNamespaceResponse{
		Created: int32(report.Created),
		Updated: int32(report.Updated),
		Skipped: int32(report.Skipped),
		Failed:  int32(report.Failed),
		DryRun:  report.DryRun,
	}

	for _, e := range report.Errors {
		resp.Errors = append(resp.Errors, &transferv1.ImportError{
			Path:      e.Path,
			Namespace: e.Namespace,
			Message:   e.Message,
		})
	}

	return connect.NewResponse(resp), nil
}
