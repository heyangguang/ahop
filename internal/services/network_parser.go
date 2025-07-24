package services

import (
	"fmt"
	"net"
	"strings"
)

// NetworkParser 网络解析器
type NetworkParser struct{}

// NewNetworkParser 创建网络解析器
func NewNetworkParser() *NetworkParser {
	return &NetworkParser{}
}

// ParseNetworks 解析网络列表，返回所有目标IP
func (p *NetworkParser) ParseNetworks(networks []string) ([]net.IP, error) {
	var allIPs []net.IP
	seenIPs := make(map[string]bool) // 去重

	for _, network := range networks {
		network = strings.TrimSpace(network)
		if network == "" {
			continue
		}

		ips, err := p.parseNetwork(network)
		if err != nil {
			return nil, fmt.Errorf("解析网络 '%s' 失败: %v", network, err)
		}

		// 去重添加
		for _, ip := range ips {
			ipStr := ip.String()
			if !seenIPs[ipStr] {
				seenIPs[ipStr] = true
				allIPs = append(allIPs, ip)
			}
		}
	}

	if len(allIPs) == 0 {
		return nil, fmt.Errorf("未解析出任何有效IP地址")
	}

	return allIPs, nil
}

// parseNetwork 解析单个网络表达式
func (p *NetworkParser) parseNetwork(network string) ([]net.IP, error) {
	// 检查是否是CIDR格式
	if strings.Contains(network, "/") {
		return p.parseCIDR(network)
	}

	// 检查是否是IP范围格式
	if strings.Contains(network, "-") {
		return p.parseIPRange(network)
	}

	// 检查是否是多个IP逗号分隔
	if strings.Contains(network, ",") {
		return p.parseMultipleIPs(network)
	}

	// 尝试解析为单个IP或主机名
	return p.parseSingleHost(network)
}

// parseCIDR 解析CIDR网段
func (p *NetworkParser) parseCIDR(cidr string) ([]net.IP, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("无效的CIDR格式: %v", err)
	}

	var ips []net.IP

	// 生成网段内的所有IP
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); {
		// 复制IP，避免引用问题
		newIP := make(net.IP, len(ip))
		copy(newIP, ip)
		ips = append(ips, newIP)

		// 递增IP地址
		for i := len(ip) - 1; i >= 0; i-- {
			ip[i]++
			if ip[i] > 0 {
				break
			}
		}
	}

	// 移除网络地址和广播地址（对于大于/30的网段）
	ones, bits := ipNet.Mask.Size()
	if bits == 32 && ones < 31 { // IPv4且不是/31或/32
		if len(ips) > 2 {
			ips = ips[1 : len(ips)-1] // 去掉第一个(网络地址)和最后一个(广播地址)
		}
	}

	return ips, nil
}

// parseIPRange 解析IP范围 (如: 192.168.1.1-192.168.1.100)
func (p *NetworkParser) parseIPRange(ipRange string) ([]net.IP, error) {
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("IP范围格式错误，应为 'start_ip-end_ip'")
	}

	startIP := net.ParseIP(strings.TrimSpace(parts[0]))
	endIP := net.ParseIP(strings.TrimSpace(parts[1]))

	if startIP == nil {
		return nil, fmt.Errorf("起始IP地址无效: %s", parts[0])
	}
	if endIP == nil {
		return nil, fmt.Errorf("结束IP地址无效: %s", parts[1])
	}

	// 确保都是IPv4或都是IPv6
	if startIP.To4() == nil && endIP.To4() != nil ||
		startIP.To4() != nil && endIP.To4() == nil {
		return nil, fmt.Errorf("起始IP和结束IP必须是同一类型(IPv4或IPv6)")
	}

	// 目前只支持IPv4范围
	if startIP.To4() == nil {
		return nil, fmt.Errorf("暂不支持IPv6范围扫描")
	}

	return p.generateIPv4Range(startIP.To4(), endIP.To4())
}

// generateIPv4Range 生成IPv4范围内的所有IP
func (p *NetworkParser) generateIPv4Range(startIP, endIP net.IP) ([]net.IP, error) {
	start := ipToUint32(startIP)
	end := ipToUint32(endIP)

	if start > end {
		return nil, fmt.Errorf("起始IP不能大于结束IP")
	}

	// 限制范围大小，防止内存溢出
	if end-start > 65535 { // 最多65536个IP
		return nil, fmt.Errorf("IP范围过大，最多支持65536个地址")
	}

	var ips []net.IP
	for i := start; i <= end; i++ {
		ip := uint32ToIP(i)
		ips = append(ips, ip)
	}

	return ips, nil
}

// parseMultipleIPs 解析逗号分隔的多个IP
func (p *NetworkParser) parseMultipleIPs(ipsStr string) ([]net.IP, error) {
	var ips []net.IP

	for _, ipStr := range strings.Split(ipsStr, ",") {
		ipStr = strings.TrimSpace(ipStr)
		if ipStr == "" {
			continue
		}

		hostIPs, err := p.parseSingleHost(ipStr)
		if err != nil {
			return nil, fmt.Errorf("解析IP '%s' 失败: %v", ipStr, err)
		}

		ips = append(ips, hostIPs...)
	}

	return ips, nil
}

// parseSingleHost 解析单个主机（IP或主机名）
func (p *NetworkParser) parseSingleHost(host string) ([]net.IP, error) {
	// 尝试直接解析为IP
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	// 尝试DNS解析
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("无法解析主机名 '%s': %v", host, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("主机名 '%s' 未解析到任何IP地址", host)
	}

	return ips, nil
}

// ParseExclusions 解析排除IP列表
func (p *NetworkParser) ParseExclusions(excludes []string) (map[string]bool, error) {
	exclusionMap := make(map[string]bool)

	for _, exclude := range excludes {
		exclude = strings.TrimSpace(exclude)
		if exclude == "" {
			continue
		}

		ips, err := p.parseNetwork(exclude)
		if err != nil {
			return nil, fmt.Errorf("解析排除项 '%s' 失败: %v", exclude, err)
		}

		for _, ip := range ips {
			exclusionMap[ip.String()] = true
		}
	}

	return exclusionMap, nil
}

// FilterExclusions 过滤排除的IP
func (p *NetworkParser) FilterExclusions(allIPs []net.IP, exclusions map[string]bool) []net.IP {
	if len(exclusions) == 0 {
		return allIPs
	}

	var filteredIPs []net.IP
	for _, ip := range allIPs {
		if !exclusions[ip.String()] {
			filteredIPs = append(filteredIPs, ip)
		}
	}

	return filteredIPs
}

// ValidatePorts 验证端口列表
func (p *NetworkParser) ValidatePorts(ports []int) error {
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("端口号 %d 无效，应在1-65535范围内", port)
		}
	}
	return nil
}

// ValidateMethods 验证扫描方法
func (p *NetworkParser) ValidateMethods(methods []string) error {
	validMethods := map[string]bool{
		"ping": true,
		"tcp":  true,
		"udp":  true,
		"arp":  true,
	}

	for _, method := range methods {
		if !validMethods[method] {
			return fmt.Errorf("不支持的扫描方法: %s，支持的方法: ping, tcp, udp, arp", method)
		}
	}

	return nil
}

// 工具函数：将IP转换为uint32
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}

// 工具函数：将uint32转换为IP
func uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// GetTargetCount 获取目标数量估计
func (p *NetworkParser) GetTargetCount(networks []string, methods []string, ports []int) (int, error) {
	ips, err := p.ParseNetworks(networks)
	if err != nil {
		return 0, err
	}

	ipCount := len(ips)

	// 计算总扫描次数
	totalScans := 0

	for _, method := range methods {
		switch method {
		case "ping", "arp":
			totalScans += ipCount // 每个IP一次扫描
		case "tcp", "udp":
			if len(ports) == 0 {
				// 默认扫描常用端口
				totalScans += ipCount * 10 // 假设10个默认端口
			} else {
				totalScans += ipCount * len(ports)
			}
		}
	}

	return totalScans, nil
}
