package scanner

import (
	"github.com/sirupsen/logrus"
)

// Scanner 主扫描器
type Scanner struct {
	ansibleScanner *AnsibleScanner
	shellScanner   *ShellScanner
	log            *logrus.Logger
}

// NewScanner 创建扫描器
func NewScanner(log *logrus.Logger) *Scanner {
	return &Scanner{
		ansibleScanner: NewAnsibleScanner(log),
		shellScanner:   NewShellScanner(log),
		log:            log,
	}
}

// ScanProject 扫描项目
func (s *Scanner) ScanProject(projectPath string) ([]ScanResult, error) {
	var results []ScanResult
	
	s.log.WithField("path", projectPath).Debug("开始扫描项目")
	
	// 1. 尝试作为 Ansible 项目扫描
	ansibleResults, err := s.ansibleScanner.Scan(projectPath)
	if err == nil && len(ansibleResults) > 0 {
		s.log.WithField("count", len(ansibleResults)).Debug("找到 Ansible 模板")
		results = append(results, ansibleResults...)
	}
	
	// 2. 扫描 Shell 脚本
	shellResults, err := s.shellScanner.Scan(projectPath)
	if err == nil && len(shellResults) > 0 {
		s.log.WithField("count", len(shellResults)).Debug("找到 Shell 脚本")
		results = append(results, shellResults...)
	}
	
	s.log.WithField("total", len(results)).Info("扫描完成")
	
	return results, nil
}