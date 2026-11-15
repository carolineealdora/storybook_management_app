package helpers

import (
	"errors"
	"net/http"

	"codebase/internal/constant/key"
	respConst "codebase/internal/constant/response"
	"codebase/models"
)

type (
	ResponseImpl struct {
		sharedHolder *models.SharedHolder
	}

	Response interface {
		CommonResponse(args models.ResponseArgs) error
	}
)

func NewResponse(sharedHolder *models.SharedHolder) *ResponseImpl {
	return &ResponseImpl{
		sharedHolder: sharedHolder,
	}
}

// CommonResponse renders `{"data":…}` on success and `{"error":{"code","message"}}` on failure.
// Internal error detail is logged, never sent to the client (unless the code is allowlisted in Wrap).
func (r *ResponseImpl) CommonResponse(args models.ResponseArgs) error {
	if args.ErrorData == nil {
		status := args.Status
		if status == 0 {
			status = http.StatusOK
		}
		return args.Ctx.JSON(status, models.DataResponse{Data: args.RespData, Meta: args.Meta})
	}

	code, info, exposeInfo := respConst.GENERAL_ERROR, args.ErrorData.Error(), false
	fields := map[string]any{"request_id": args.Ctx.Request().Context().Value(key.RequestId)}
	var entry *LogEntry
	if errors.As(args.ErrorData, &entry) {
		code, info, exposeInfo = entry.ErrConst, entry.ErrInfo, entry.ErrWrap
		fields["source"] = map[string]any{"component": entry.Component, "src_file": entry.SrcFile, "src_func": entry.SrcFunc}
	}
	payload, ok := respConst.Responses[code]
	if !ok {
		code, payload = respConst.GENERAL_ERROR, respConst.Responses[respConst.GENERAL_ERROR]
	}
	message := payload.Desc
	if exposeInfo && info != "" {
		message = info
	}

	if r.sharedHolder != nil && r.sharedHolder.Log != nil {
		fields["code"], fields["error"] = code, info
		r.sharedHolder.Log.WithFields(fields).Error("onError")
	}

	return args.Ctx.JSON(payload.HttpCode, models.ErrorResponse{Error: models.ErrorBody{Code: code, Message: message}})
}
