package models

import (
	"context"
	"net"
	"sync"
	"time"
)

// ScanTask 扫描任务
type ScanTask struct {
	ScanID      string              `json:"scan_id"`
	TenantID    uint               `json:"tenant_id"`
	UserID      uint               `json:"user_id"`
	Username    string             `json:"username"`
	Config      *ScanConfig        `json:"config"`
	Status      string             `json:"status"` // running/completed/cancelled/error
	Results     []*ScanResult      `json:"results"`
	Progress    int                `json:"progress"` // 0-100
	StartTime   time.Time          `json:"start_time"`
	EndTime     *time.Time         `json:"end_time,omitempty"`
	Error       string             `json:"error,omitempty"`
	Context     context.Context    `json:"-"`
	CancelFunc  context.CancelFunc `json:"-"`
	mu          sync.RWMutex       `json:"-"`
}

// ScanConfig 扫描配置
type ScanConfig struct {
	// 目标网络
	Networks    []string `json:"networks" binding:"required"`        // CIDR、IP范围、多网段
	ExcludeIPs  []string `json:"exclude_ips"`                       // 排除IP列表
	
	// 扫描协议
	Methods     []string `json:"methods" binding:"required"`         // ping/tcp/udp/arp
	Ports       []int    `json:"ports"`                             // TCP/UDP端口列表
	
	// 性能参数
	Timeout     int      `json:"timeout"`                           // 单个目标超时(秒)
	Concurrency int      `json:"concurrency"`                       // 并发数
	
	// 扩展功能
	OSDetection bool     `json:"os_detection"`                      // 操作系统检测
}

// ScanResult 扫描结果
type ScanResult struct {
	IP          string        `json:"ip"`
	Port        int           `json:"port,omitempty"`         // 端口（TCP/UDP扫描时有效）
	Protocol    string        `json:"protocol"`               // ping/tcp/udp/arp
	Status      string        `json:"status"`                 // alive/dead/timeout/error
	RTT         time.Duration `json:"rtt"`                    // 响应时间
	OSInfo      string        `json:"os_info,omitempty"`      // 操作系统信息
	Timestamp   time.Time     `json:"timestamp"`              // 扫描时间
	Error       string        `json:"error,omitempty"`        // 错误信息
}

// WebSocket消息类型
const (
	WSMessageTypeProgress = "progress"
	WSMessageTypeResult   = "result"
	WSMessageTypeComplete = "complete"
	WSMessageTypeError    = "error"
)

// WSMessage WebSocket消息
type WSMessage struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	ScanID string      `json:"scan_id"`
}

// ProgressData 进度数据
type ProgressData struct {
	Progress int    `json:"progress"`    // 0-100
	Message  string `json:"message"`     // 当前状态描述
	Total    int    `json:"total"`       // 总目标数
	Scanned  int    `json:"scanned"`     // 已扫描数
	Found    int    `json:"found"`       // 发现存活数
}

// ResultData 扫描结果数据
type ResultData struct {
	IP      string        `json:"ip"`
	Results []*ScanResult `json:"results"` // 该IP的所有协议扫描结果
}

// CompleteData 完成数据
type CompleteData struct {
	ScanID     string        `json:"scan_id"`
	TotalFound int           `json:"total_found"`
	Duration   string        `json:"duration"`
	Results    []*ScanResult `json:"results"`
}

// ErrorData 错误数据
type ErrorData struct {
	ScanID  string `json:"scan_id"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

// NetworkScanImportRequest 网络扫描批量导入请求
type NetworkScanImportRequest struct {
	ScanID       string `json:"scan_id" binding:"required"`
	IPs          []string `json:"ips" binding:"required"`           // 选择的IP列表
	CredentialID uint   `json:"credential_id" binding:"required"`   // 凭证ID
	HostGroupID  *uint  `json:"host_group_id"`                     // 可选主机组ID
	Tags         []uint `json:"tags"`                              // 可选标签ID列表
	Port         int    `json:"port"`                              // SSH端口，默认22
	Description  string `json:"description"`                       // 可选描述（默认"网络扫描发现"）
}

// 扫描状态常量
const (
	ScanStatusRunning   = "running"
	ScanStatusCompleted = "completed"
	ScanStatusCancelled = "cancelled"
	ScanStatusError     = "error"
)

// 扫描协议常量
const (
	ScanMethodPing = "ping"
	ScanMethodTCP  = "tcp"
	ScanMethodUDP  = "udp"
	ScanMethodARP  = "arp"
)

// 扫描结果状态常量
const (
	ScanResultStatusAlive   = "alive"
	ScanResultStatusDead    = "dead"
	ScanResultStatusTimeout = "timeout"
	ScanResultStatusError   = "error"
)

// AddResult 添加扫描结果（线程安全）
func (st *ScanTask) AddResult(result *ScanResult) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Results = append(st.Results, result)
}

// GetResults 获取扫描结果（线程安全）
func (st *ScanTask) GetResults() []*ScanResult {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	// 返回副本，避免并发修改
	results := make([]*ScanResult, len(st.Results))
	copy(results, st.Results)
	return results
}

// UpdateProgress 更新进度（线程安全）
func (st *ScanTask) UpdateProgress(progress int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Progress = progress
}

// GetProgress 获取进度（线程安全）
func (st *ScanTask) GetProgress() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.Progress
}

// SetStatus 设置状态（线程安全）
func (st *ScanTask) SetStatus(status string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Status = status
}

// GetStatus 获取状态（线程安全）
func (st *ScanTask) GetStatus() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.Status
}

// SetError 设置错误（线程安全）
func (st *ScanTask) SetError(err string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Error = err
}

// GetError 获取错误（线程安全）
func (st *ScanTask) GetError() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.Error
}

// GetAliveCount 获取存活主机数量
func (st *ScanTask) GetAliveCount() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	count := 0
	for _, result := range st.Results {
		if result.Status == ScanResultStatusAlive {
			count++
		}
	}
	return count
}

// GetAliveIPs 获取存活主机IP列表
func (st *ScanTask) GetAliveIPs() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	ipSet := make(map[string]bool)
	for _, result := range st.Results {
		if result.Status == ScanResultStatusAlive {
			ipSet[result.IP] = true
		}
	}
	
	var ips []string
	for ip := range ipSet {
		ips = append(ips, ip)
	}
	return ips
}

// ValidateIP 验证IP格式
func ValidateIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// ParseCIDR 解析CIDR网段
func ParseCIDR(cidr string) ([]net.IP, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	
	var ips []net.IP
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); {
		ips = append(ips, net.IP(make([]byte, len(ip))))
		copy(ips[len(ips)-1], ip)
		
		// 递增IP地址
		for i := len(ip) - 1; i >= 0; i-- {
			ip[i]++
			if ip[i] > 0 {
				break
			}
		}
	}
	
	// 移除网络地址和广播地址（对于/24及以上网段）
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}
	
	return ips, nil
}