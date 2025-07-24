package pagination

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

// PageParams 分页参数
type PageParams struct {
	Page     int `json:"page" form:"page"`
	PageSize int `json:"page_size" form:"page_size"`
}

// PageInfo 分页信息
type PageInfo struct {
	Page       int   `json:"page"`        // 当前页
	PageSize   int   `json:"page_size"`   // 每页大小
	Total      int64 `json:"total"`       // 总记录数
	TotalPages int   `json:"total_pages"` // 总页数
	HasNext    bool  `json:"has_next"`    // 是否有下一页
	HasPrev    bool  `json:"has_prev"`    // 是否有上一页
}

// 分页配置
const (
	DefaultPage     = 1
	DefaultPageSize = 10
	MaxPageSize     = 100
)

// ParsePageParams 从请求中解析分页参数
func ParsePageParams(c *gin.Context) *PageParams {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = DefaultPage
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	return &PageParams{
		Page:     page,
		PageSize: pageSize,
	}
}

// NewPageInfo 计算分页信息
func NewPageInfo(page, pageSize int, total int64) *PageInfo {
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return &PageInfo{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// GetOffset 计算offset
func (p *PageParams) GetOffset() int {
	return (p.Page - 1) * p.PageSize
}

// GetLimit 计算limit
func (p *PageParams) GetLimit() int {
	return p.PageSize
}
