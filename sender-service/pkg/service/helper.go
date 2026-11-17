package service

import (
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/spf13/cast"
	"context"
)

const (
	defaultCurrentPage = 1
	defaultPageSize    = 10
	maxPageSize        = 500
)

func limitAndOffset(currentPage, pageSize int64) (limit, offset int) {
	if currentPage <= 0 {
		currentPage = defaultCurrentPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	} else if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return int(pageSize), int((currentPage - 1) * pageSize)
}

func pageOf(currentPage, pageSize, total int64) *common_v1.Page {
	if currentPage <= 0 {
		currentPage = defaultCurrentPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	} else if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}
	return &common_v1.Page{
		CurrentPage: currentPage,
		PageSize:    pageSize,
		Total:       total,
		TotalPages:  totalPages,
	}
}

// operatorFromContext 取当前操作者（审计字段服务端注入，宪法 XII）。
// 身份由入站 authz 拦截器注入 ctx（仅审计用）；无身份时为空串。
func operatorFromContext(ctx context.Context) string {
	return cast.ToString(authz.GetAccountId(ctx))
}
