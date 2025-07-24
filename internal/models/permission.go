package models

// Permission 权限模型
type Permission struct {
	BaseModel
	Code        string `gorm:"uniqueIndex;size:100;not null" json:"code"` // 权限代码，如 "user:create"
	Name        string `gorm:"size:100;not null" json:"name"`             // 权限名称，如 "创建用户"
	Description string `gorm:"size:255" json:"description"`               // 权限描述
	Module      string `gorm:"size:50;not null" json:"module"`            // 所属模块，如 "user", "tenant"
	Action      string `gorm:"size:50;not null" json:"action"`            // 操作类型，如 "create", "read"
}

// 权限模块常量
const (
	ModuleTenant = "tenant" // 租户管理
	ModuleUser   = "user"   // 用户管理
	ModuleRole   = "role"   // 角色管理
)

// 权限操作常量
const (
	ActionCreate = "create" // 创建
	ActionRead   = "read"   // 读取
	ActionUpdate = "update" // 更新
	ActionDelete = "delete" // 删除
	ActionList   = "list"   // 列表
)
