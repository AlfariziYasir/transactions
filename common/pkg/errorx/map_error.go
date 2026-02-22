package errorx

import (
	"errors"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func MapError(err error, log *logger.Logger) error {
	if err == nil {
		return nil
	}

	if appErr, ok := errors.AsType[*AppError](err); ok {
		if appErr.Err != nil {
			log.Error(appErr.Message, zap.Error(appErr.Err))
		}

		switch appErr.Type {
		case ErrTypeValidation:
			st := status.New(codes.InvalidArgument, appErr.Message)
			br := &errdetails.BadRequest{}
			for field, desc := range appErr.Fields {
				br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
					Field:       field,
					Description: desc,
				})
			}

			stWithDetails, _ := st.WithDetails(br)
			return stWithDetails.Err()

		case ErrTypeConflict:
			return status.Error(codes.AlreadyExists, appErr.Message)

		case ErrTypeNotFound:
			return status.Error(codes.NotFound, appErr.Message)

		case ErrTypeUnauthorized:
			return status.Error(codes.Unauthenticated, appErr.Message)

		case ErrTypeInternal:
			return status.Error(codes.Internal, "internal system error")
		}
	}

	return status.Error(codes.Unknown, "unknown error occurred")
}
