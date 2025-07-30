package models

import (
	"encoding/json"
	"time"
)

// HealingRuleExecution 自愈规则执行记录
type HealingRuleExecution struct {
	ID                 uint            `gorm:"primarykey" json:"id"`
	TenantID           uint            `gorm:"not null;index" json:"tenant_id"`
	RuleID             uint            `gorm:"not null;index:idx_rule_exec_rule_time" json:"rule_id"`
	ExecutionTime      time.Time       `gorm:"not null;index:idx_rule_exec_rule_time" json:"execution_time"`
	
	// 扫描统计
	TotalTicketsScanned int            `gorm:"default:0" json:"total_tickets_scanned"`
	MatchedTickets      int            `gorm:"default:0" json:"matched_tickets"`
	ExecutionsCreated   int            `gorm:"default:0" json:"executions_created"`
	
	// 匹配的工单列表
	MatchedTicketIDs    json.RawMessage `gorm:"type:json" json:"matched_ticket_ids,omitempty"`
	
	// 创建的执行列表
	ExecutionIDs        json.RawMessage `gorm:"type:json" json:"execution_ids,omitempty"`
	
	// 执行结果
	Status              string          `gorm:"type:varchar(50)" json:"status"` // success/partial/failed
	ErrorMsg            string          `gorm:"type:text" json:"error_msg,omitempty"`
	Duration            int             `gorm:"comment:执行耗时(毫秒)" json:"duration"`
	
	CreatedAt           time.Time       `json:"created_at"`
	
	// 关联
	Tenant              Tenant          `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Rule                HealingRule     `gorm:"foreignKey:RuleID" json:"rule,omitempty"`
}

// TableName 指定表名
func (HealingRuleExecution) TableName() string {
	return "healing_rule_executions"
}

// 执行状态常量
const (
	RuleExecStatusSuccess = "success"  // 全部成功
	RuleExecStatusPartial = "partial"  // 部分成功
	RuleExecStatusFailed  = "failed"   // 全部失败
	RuleExecStatusNoMatch = "no_match" // 没有匹配到工单
)

// MatchedTicketInfo 匹配的工单信息
type MatchedTicketInfo struct {
	ID         uint   `json:"id"`
	ExternalID string `json:"external_id"`
	Title      string `json:"title"`
	Priority   string `json:"priority"`
	Status     string `json:"status"`
}

// SetMatchedTickets 设置匹配的工单信息
func (r *HealingRuleExecution) SetMatchedTickets(tickets []MatchedTicketInfo) error {
	if len(tickets) == 0 {
		r.MatchedTicketIDs = nil
		return nil
	}
	
	data, err := json.Marshal(tickets)
	if err != nil {
		return err
	}
	r.MatchedTicketIDs = data
	return nil
}

// GetMatchedTickets 获取匹配的工单信息
func (r *HealingRuleExecution) GetMatchedTickets() ([]MatchedTicketInfo, error) {
	if r.MatchedTicketIDs == nil {
		return []MatchedTicketInfo{}, nil
	}
	
	var tickets []MatchedTicketInfo
	err := json.Unmarshal(r.MatchedTicketIDs, &tickets)
	return tickets, err
}

// SetExecutionIDs 设置创建的执行ID列表
func (r *HealingRuleExecution) SetExecutionIDs(ids []string) error {
	if len(ids) == 0 {
		r.ExecutionIDs = nil
		return nil
	}
	
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	r.ExecutionIDs = data
	return nil
}

// GetExecutionIDs 获取创建的执行ID列表
func (r *HealingRuleExecution) GetExecutionIDs() ([]string, error) {
	if r.ExecutionIDs == nil {
		return []string{}, nil
	}
	
	var ids []string
	err := json.Unmarshal(r.ExecutionIDs, &ids)
	return ids, err
}

// RuleExecutionStats 规则执行统计
type RuleExecutionStats struct {
	RuleID             uint    `json:"rule_id"`
	RuleName           string  `json:"rule_name"`
	TotalExecutions    int64   `json:"total_executions"`
	SuccessExecutions  int64   `json:"success_executions"`
	FailedExecutions   int64   `json:"failed_executions"`
	TotalTicketsScanned int64  `json:"total_tickets_scanned"`
	TotalMatched       int64   `json:"total_matched"`
	TotalCreated       int64   `json:"total_created"`
	AvgDuration        float64 `json:"avg_duration"` // 平均执行时间（毫秒）
	SuccessRate        float64 `json:"success_rate"` // 成功率
	MatchRate          float64 `json:"match_rate"`   // 匹配率
	LastExecutionTime  *time.Time `json:"last_execution_time"`
}