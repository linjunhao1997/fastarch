package helper

import (
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	"iam-service/pkg/constant"
	"math"
)

func SetDefaultPageParam(currentPage, pageSize int64) (int64, int64) {
	if currentPage <= 0 {
		currentPage = constant.DefaultCurrentPage
	}
	if pageSize <= 0 {
		pageSize = constant.DefaultPageSize
	} else if pageSize > constant.DefaultMaxPageSize {
		pageSize = constant.DefaultMaxPageSize
	}

	return currentPage, pageSize
}

func GetLimitAndOffset(currentPage, pageSize int64) (limit, offset int) {
	currentPage, pageSize = SetDefaultPageParam(currentPage, pageSize)
	return int(pageSize), int(currentPage-1) * int(pageSize)
}

func GetPage(currentPage, pageSize, total int64) *common_v1.Page {
	currentPage, pageSize = SetDefaultPageParam(currentPage, pageSize)

	// 求总页数
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return &common_v1.Page{
		Total:       total,
		CurrentPage: currentPage,
		PageSize:    pageSize,
		TotalPages:  int64(totalPages),
	}
}
