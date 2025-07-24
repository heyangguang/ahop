package services

import (
	"ahop/internal/models"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ScannerInterface 扫描器接口
type ScannerInterface interface {
	Scan(ctx context.Context, ip net.IP) []*models.ScanResult
	GetProtocol() string
}

// PingScanner PING扫描器
type PingScanner struct {
	timeout time.Duration
}

// NewPingScanner 创建PING扫描器
func NewPingScanner(timeout time.Duration) *PingScanner {
	return &PingScanner{
		timeout: timeout,
	}
}

// GetProtocol 获取协议名称
func (s *PingScanner) GetProtocol() string {
	return models.ScanMethodPing
}

// Scan 执行PING扫描
func (s *PingScanner) Scan(ctx context.Context, ip net.IP) []*models.ScanResult {
	startTime := time.Now()

	result := &models.ScanResult{
		IP:        ip.String(),
		Protocol:  models.ScanMethodPing,
		Timestamp: startTime,
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		result.Status = models.ScanResultStatusError
		result.Error = "扫描已取消"
		return []*models.ScanResult{result}
	default:
	}

	// 创建带超时的上下文
	pingCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// 执行ping命令
	success, rtt, err := s.ping(pingCtx, ip.String())

	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			result.Status = models.ScanResultStatusTimeout
		} else {
			result.Status = models.ScanResultStatusError
			result.Error = err.Error()
		}
	} else if success {
		result.Status = models.ScanResultStatusAlive
		result.RTT = rtt
	} else {
		result.Status = models.ScanResultStatusDead
	}

	return []*models.ScanResult{result}
}

// ping 执行系统ping命令
func (s *PingScanner) ping(ctx context.Context, ip string) (bool, time.Duration, error) {
	// 使用系统ping命令，发送1个包
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", fmt.Sprintf("%.0f", s.timeout.Seconds()), ip)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return false, 0, fmt.Errorf("ping timeout")
	}

	if err != nil {
		// ping命令失败，但可能是主机不可达（正常情况）
		if strings.Contains(outputStr, "Destination Host Unreachable") ||
			strings.Contains(outputStr, "Network is unreachable") {
			return false, 0, nil
		}
		// 其他错误
		return false, 0, fmt.Errorf("ping执行失败: %v", err)
	}

	// 解析ping结果，提取RTT
	rtt := s.parseRTT(outputStr)
	return true, rtt, nil
}

// parseRTT 从ping输出中解析RTT时间
func (s *PingScanner) parseRTT(output string) time.Duration {
	// 查找time=xxx.xxx格式
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "time=") {
			// 示例: 64 bytes from 192.168.1.1: icmp_seq=1 ttl=64 time=1.23 ms
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.HasPrefix(part, "time=") {
					timeStr := strings.TrimPrefix(part, "time=")
					if i+1 < len(parts) && parts[i+1] == "ms" {
						if ms, err := strconv.ParseFloat(timeStr, 64); err == nil {
							return time.Duration(ms * float64(time.Millisecond))
						}
					}
				}
			}
		}
	}
	return 0
}

// TCPScanner TCP端口扫描器
type TCPScanner struct {
	timeout time.Duration
	ports   []int
}

// NewTCPScanner 创建TCP扫描器
func NewTCPScanner(timeout time.Duration, ports []int) *TCPScanner {
	// 如果没有指定端口，使用常用端口
	if len(ports) == 0 {
		ports = []int{22, 23, 25, 53, 80, 110, 443, 993, 995, 3389} // 常用端口
	}

	return &TCPScanner{
		timeout: timeout,
		ports:   ports,
	}
}

// GetProtocol 获取协议名称
func (s *TCPScanner) GetProtocol() string {
	return models.ScanMethodTCP
}

// Scan 执行TCP端口扫描
func (s *TCPScanner) Scan(ctx context.Context, ip net.IP) []*models.ScanResult {
	var results []*models.ScanResult

	for _, port := range s.ports {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			result := &models.ScanResult{
				IP:        ip.String(),
				Port:      port,
				Protocol:  models.ScanMethodTCP,
				Status:    models.ScanResultStatusError,
				Error:     "扫描已取消",
				Timestamp: time.Now(),
			}
			results = append(results, result)
			continue
		default:
		}

		result := s.scanTCPPort(ctx, ip.String(), port)
		results = append(results, result)
	}

	return results
}

// scanTCPPort 扫描单个TCP端口
func (s *TCPScanner) scanTCPPort(ctx context.Context, ip string, port int) *models.ScanResult {
	startTime := time.Now()

	result := &models.ScanResult{
		IP:        ip,
		Port:      port,
		Protocol:  models.ScanMethodTCP,
		Timestamp: startTime,
	}

	// 创建带超时的拨号器
	dialer := &net.Dialer{
		Timeout: s.timeout,
	}

	// 尝试连接
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ip, port))
	rtt := time.Since(startTime)
	result.RTT = rtt

	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			result.Status = models.ScanResultStatusTimeout
		} else if strings.Contains(err.Error(), "connection refused") {
			result.Status = models.ScanResultStatusDead
		} else {
			result.Status = models.ScanResultStatusError
			result.Error = err.Error()
		}
	} else {
		result.Status = models.ScanResultStatusAlive
		conn.Close()
	}

	return result
}

// UDPScanner UDP端口扫描器
type UDPScanner struct {
	timeout time.Duration
	ports   []int
}

// NewUDPScanner 创建UDP扫描器
func NewUDPScanner(timeout time.Duration, ports []int) *UDPScanner {
	// 如果没有指定端口，使用常用UDP端口
	if len(ports) == 0 {
		ports = []int{53, 67, 68, 69, 123, 161, 162, 514} // 常用UDP端口
	}

	return &UDPScanner{
		timeout: timeout,
		ports:   ports,
	}
}

// GetProtocol 获取协议名称
func (s *UDPScanner) GetProtocol() string {
	return models.ScanMethodUDP
}

// Scan 执行UDP端口扫描
func (s *UDPScanner) Scan(ctx context.Context, ip net.IP) []*models.ScanResult {
	var results []*models.ScanResult

	for _, port := range s.ports {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			result := &models.ScanResult{
				IP:        ip.String(),
				Port:      port,
				Protocol:  models.ScanMethodUDP,
				Status:    models.ScanResultStatusError,
				Error:     "扫描已取消",
				Timestamp: time.Now(),
			}
			results = append(results, result)
			continue
		default:
		}

		result := s.scanUDPPort(ctx, ip.String(), port)
		results = append(results, result)
	}

	return results
}

// scanUDPPort 扫描单个UDP端口（注意：UDP扫描较难确定端口状态）
func (s *UDPScanner) scanUDPPort(ctx context.Context, ip string, port int) *models.ScanResult {
	startTime := time.Now()

	result := &models.ScanResult{
		IP:        ip,
		Port:      port,
		Protocol:  models.ScanMethodUDP,
		Timestamp: startTime,
	}

	// 创建UDP连接
	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:%d", ip, port), s.timeout)
	rtt := time.Since(startTime)
	result.RTT = rtt

	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			result.Status = models.ScanResultStatusTimeout
		} else {
			result.Status = models.ScanResultStatusError
			result.Error = err.Error()
		}
	} else {
		// UDP连接成功不代表端口开放，只能说主机可达
		result.Status = models.ScanResultStatusAlive
		conn.Close()
	}

	return result
}

// ARPScanner ARP扫描器（仅限本地网段）
type ARPScanner struct {
	timeout time.Duration
}

// NewARPScanner 创建ARP扫描器
func NewARPScanner(timeout time.Duration) *ARPScanner {
	return &ARPScanner{
		timeout: timeout,
	}
}

// GetProtocol 获取协议名称
func (s *ARPScanner) GetProtocol() string {
	return models.ScanMethodARP
}

// Scan 执行ARP扫描
func (s *ARPScanner) Scan(ctx context.Context, ip net.IP) []*models.ScanResult {
	startTime := time.Now()

	result := &models.ScanResult{
		IP:        ip.String(),
		Protocol:  models.ScanMethodARP,
		Timestamp: startTime,
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		result.Status = models.ScanResultStatusError
		result.Error = "扫描已取消"
		return []*models.ScanResult{result}
	default:
	}

	// 使用arping命令（需要安装）
	success, rtt, err := s.arping(ctx, ip.String())

	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			result.Status = models.ScanResultStatusTimeout
		} else {
			result.Status = models.ScanResultStatusError
			result.Error = err.Error()
		}
	} else if success {
		result.Status = models.ScanResultStatusAlive
		result.RTT = rtt
	} else {
		result.Status = models.ScanResultStatusDead
	}

	return []*models.ScanResult{result}
}

// arping 执行ARP ping
func (s *ARPScanner) arping(ctx context.Context, ip string) (bool, time.Duration, error) {
	// 创建带超时的上下文
	arpCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// 尝试使用arping命令
	cmd := exec.CommandContext(arpCtx, "arping", "-c", "1", "-W", "1", ip)
	startTime := time.Now()

	output, err := cmd.CombinedOutput()
	rtt := time.Since(startTime)

	if arpCtx.Err() == context.DeadlineExceeded {
		return false, 0, fmt.Errorf("arping timeout")
	}

	if err != nil {
		// 如果arping命令不存在，回退到ping
		if strings.Contains(err.Error(), "executable file not found") {
			return s.fallbackToPing(ctx, ip)
		}
		return false, 0, fmt.Errorf("arping执行失败: %v", err)
	}

	// 检查输出是否显示成功
	outputStr := string(output)
	if strings.Contains(outputStr, "Received 1 response") ||
		strings.Contains(outputStr, "1 packets received") {
		return true, rtt, nil
	}

	return false, 0, nil
}

// fallbackToPing ARP失败时回退到ping
func (s *ARPScanner) fallbackToPing(ctx context.Context, ip string) (bool, time.Duration, error) {
	pingScanner := NewPingScanner(s.timeout)
	results := pingScanner.Scan(ctx, net.ParseIP(ip))

	if len(results) > 0 {
		result := results[0]
		return result.Status == models.ScanResultStatusAlive, result.RTT, nil
	}

	return false, 0, fmt.Errorf("ping fallback失败")
}
