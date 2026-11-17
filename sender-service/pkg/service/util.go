package service

import (
	"errors"

	"github.com/spf13/cast"
	"gorm.io/gorm"
)

func toString(id int64) string {
	return cast.ToString(id)
}

// translateNotFound 把 gorm 记录不存在转成本域业务错误，其余原样透传。
func translateNotFound(err error, notFound *Error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound
	}
	return err
}
