package v2

import (
	"context"

	"connectrpc.com/connect"

	transferv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1"
	transferuc "github.com/sergeyslonimsky/elara/internal/usecase/transfer"
)

type TransferHandler struct {
	exportNamespace *transferuc.ExportNamespaceUseCase
	exportAll       *transferuc.ExportAllUseCase
	importNamespace *transferuc.ImportNamespaceUseCase
}

func NewTransferHandler(
	exportNamespace *transferuc.ExportNamespaceUseCase,
	exportAll *transferuc.ExportAllUseCase,
	importNamespace *transferuc.ImportNamespaceUseCase,
) *TransferHandler {
	return &TransferHandler{
		exportNamespace: exportNamespace,
		exportAll:       exportAll,
		importNamespace: importNamespace,
	}
}

func (h *TransferHandler) ExportNamespace(
	ctx context.Context,
	req *connect.Request[transferv1.ExportNamespaceRequest],
) (*connect.Response[transferv1.ExportNamespaceResponse], error) {
	data, ct, filename, err := h.exportNamespace.Execute(
		ctx,
		req.Msg.GetNamespace(),
		req.Msg.GetZip(),
		req.Msg.GetEncoding(),
	)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&transferv1.ExportNamespaceResponse{
		Data:        data,
		ContentType: ct,
		Filename:    filename,
	}), nil
}

func (h *TransferHandler) ExportAll(
	ctx context.Context,
	req *connect.Request[transferv1.ExportAllRequest],
) (*connect.Response[transferv1.ExportAllResponse], error) {
	data, ct, filename, err := h.exportAll.Execute(
		ctx,
		req.Msg.GetZip(),
		req.Msg.GetEncoding(),
		req.Msg.GetZipLayout(),
	)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&transferv1.ExportAllResponse{
		Data:        data,
		ContentType: ct,
		Filename:    filename,
	}), nil
}

func (h *TransferHandler) ImportNamespace(
	ctx context.Context,
	req *connect.Request[transferv1.ImportNamespaceRequest],
) (*connect.Response[transferv1.ImportNamespaceResponse], error) {
	report, err := h.importNamespace.Execute(
		ctx,
		req.Msg.GetData(),
		req.Msg.GetOnConflict(),
		req.Msg.GetDryRun(),
		req.Msg.GetNamespace(),
	)
	if err != nil {
		return nil, toConnectError(err)
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
