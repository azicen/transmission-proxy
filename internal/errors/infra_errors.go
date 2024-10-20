package errors

import (
	"fmt"

	errors "github.com/go-kratos/kratos/v2/errors"
)

const (
	ErrReasonResourceNotExist string = "ERR_RESOURCE_NOT_EXIST"
	ErrCodeResourceNotExist   int32  = 404
)

func IsResourceNotExist(err error) bool {
	if err == nil {
		return false
	}
	e := errors.FromError(err)
	return e.Reason == ErrReasonResourceNotExist && e.Code == ErrCodeResourceNotExist
}

func ResourceNotExist(format string, args ...interface{}) *errors.Error {
	return errors.New(
		int(ErrCodeResourceNotExist),
		ErrReasonResourceNotExist,
		fmt.Sprintf(format, args...),
	)
}
