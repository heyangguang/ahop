package models

import (
	"time"
)

// HealingRule 自愈规则
type HealingRule struct {
	BaseModel
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// 基本信息
	Name        string `gorm:"size:200;not null" json:"name"`
	Code        string `gorm:"size:100;not null;uniqueIndex:idx_tenant_code" json:"code"`
	Description string `gorm:"size:500" json:"description"`
	IsActive    bool   `gorm:"default:true;index" json:"is_active"`

	// 触发配置
	TriggerType string `gorm:"size:20;not null" json:"trigger_type"` // scheduled/manual
	CronExpr    string `gorm:"size:100" json:"cron_expr"`            // 定时触发的cron表达式

	// 匹配条件
	MatchRules JSON `gorm:"type:jsonb;not null" json:"match_rules"` // 匹配规则JSON
	Priority   int  `gorm:"default:100" json:"priority"`             // 优先级，数字越小优先级越高

	// 关联工作流
	WorkflowID uint `gorm:"not null;index" json:"workflow_id"`

	// 执行控制
	MaxExecutions   int `gorm:"default:0" json:"max_executions"`    // 最大执行次数，0表示不限制
	CooldownMinutes int `gorm:"default:30" json:"cooldown_minutes"` // 冷却时间（分钟）

	// 执行统计
	LastExecuteAt *time.Time `json:"last_execute_at"`
	NextRunAt     *time.Time `gorm:"index" json:"next_run_at"`         // 下次执行时间
	ExecuteCount  int64      `gorm:"default:0" json:"execute_count"`
	SuccessCount  int64      `gorm:"default:0" json:"success_count"`
	FailureCount  int64      `gorm:"default:0" json:"failure_count"`

	// 审计
	CreatedBy uint `gorm:"not null" json:"created_by"`
	UpdatedBy uint `json:"updated_by"`

	// 关联
	Workflow HealingWorkflow `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	Tenant   Tenant          `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// TableName 指定表名
func (HealingRule) TableName() string {
	return "healing_rules"
}

// HealingRuleMatchCondition 匹配规则结构
type HealingRuleMatchCondition struct {
	Source     string                 `json:"source"`     // ticket/host/metric
	Field      string                 `json:"field"`      // 字段名
	Operator   string                 `json:"operator"`   // equals/contains/regex/gt/lt
	Value      interface{}            `json:"value"`      // 匹配值
	LogicOp    string                 `json:"logic_op"`   // and/or
	Conditions []HealingRuleMatchCondition `json:"conditions"` // 嵌套条件
}

// 触发类型常量
const (
	HealingTriggerScheduled = "scheduled" // 定时触发
	HealingTriggerManual    = "manual"    // 手动触发
)

// 匹配操作符常量
const (
	MatchOperatorEquals   = "equals"
	MatchOperatorContains = "contains"
	MatchOperatorRegex    = "regex"
	MatchOperatorGT       = "gt"
	MatchOperatorLT       = "lt"
	MatchOperatorGTE      = "gte"
	MatchOperatorLTE      = "lte"
	MatchOperatorIn       = "in"
	MatchOperatorNotIn    = "not_in"
)