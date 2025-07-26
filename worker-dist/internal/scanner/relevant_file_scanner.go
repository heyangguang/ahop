package scanner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// RelevantFileNode 相关文件节点（增强版）
type RelevantFileNode struct {
	ID         string              `json:"id"`                  // 唯一标识符
	Name       string              `json:"name"`                // 文件或目录名
	Path       string              `json:"path"`                // 相对于仓库根目录的路径
	Type       string              `json:"type"`                // file/directory
	FileType   string              `json:"file_type,omitempty"` // 文件类型：ansible/shell/template/survey
	Size       int64               `json:"size"`                // 文件大小（字节）
	Selectable bool                `json:"selectable"`          // 是否可选（只有文件可选）
	Children   []RelevantFileNode  `json:"children,omitempty"`  // 子节点（目录才有）
}

// RelevantFileScanner 相关文件扫描器（只扫描自动化相关文件）
type RelevantFileScanner struct {
	log            *logrus.Logger
	ignorePatterns []string
	maxDepth       int
	nodeIDCounter  int
}

// NewRelevantFileScanner 创建相关文件扫描器
func NewRelevantFileScanner(log *logrus.Logger) *RelevantFileScanner {
	return &RelevantFileScanner{
		log: log,
		ignorePatterns: []string{
			".git",
			".svn",
			".hg",
			"node_modules",
			"__pycache__",
			".pytest_cache",
			".venv",
			"venv",
			".idea",
			".vscode",
			"vendor",
			"dist",
			"build",
		},
		maxDepth:      10,
		nodeIDCounter: 0,
	}
}

// ScanRelevantFiles 扫描相关文件
func (s *RelevantFileScanner) ScanRelevantFiles(rootPath string) (*RelevantFileNode, error) {
	s.log.WithField("path", rootPath).Info("开始扫描相关文件")
	s.nodeIDCounter = 0 // 重置ID计数器

	// 检查根目录
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, os.ErrInvalid
	}

	// 创建根节点
	root := &RelevantFileNode{
		ID:         s.generateNodeID(),
		Name:       filepath.Base(rootPath),
		Path:       ".",
		Type:       "directory",
		Size:       0,
		Selectable: false,
		Children:   []RelevantFileNode{},
	}

	// 递归扫描
	if err := s.scanDirectory(rootPath, rootPath, root, 0); err != nil {
		s.log.WithError(err).Warn("扫描目录时出现错误")
	}

	// 清理空目录
	s.removeEmptyDirectories(root)

	s.log.WithField("path", rootPath).Info("相关文件扫描完成")
	return root, nil
}

// scanDirectory 递归扫描目录
func (s *RelevantFileScanner) scanDirectory(rootPath, currentPath string, parent *RelevantFileNode, depth int) error {
	// 检查递归深度
	if depth > s.maxDepth {
		s.log.WithField("path", currentPath).Warn("达到最大递归深度，跳过")
		return nil
	}

	// 读取目录内容
	files, err := ioutil.ReadDir(currentPath)
	if err != nil {
		s.log.WithError(err).WithField("path", currentPath).Warn("读取目录失败")
		return err
	}

	for _, file := range files {
		// 检查是否应该忽略
		if s.shouldIgnore(file.Name()) {
			continue
		}

		fullPath := filepath.Join(currentPath, file.Name())
		relPath, err := filepath.Rel(rootPath, fullPath)
		if err != nil {
			s.log.WithError(err).WithField("path", fullPath).Warn("计算相对路径失败")
			continue
		}

		if file.IsDir() {
			// 创建目录节点（稍后会清理空目录）
			node := RelevantFileNode{
				ID:         s.generateNodeID(),
				Name:       file.Name(),
				Path:       relPath,
				Type:       "directory",
				Size:       0,
				Selectable: false,
				Children:   []RelevantFileNode{},
			}

			// 递归扫描子目录
			if err := s.scanDirectory(rootPath, fullPath, &node, depth+1); err != nil {
				s.log.WithError(err).WithField("path", fullPath).Debug("扫描子目录失败")
			}

			// 即使目录为空也先添加，后面会清理
			parent.Children = append(parent.Children, node)
		} else if s.isRelevantFile(file.Name()) {
			// 只添加相关文件
			nodeID := s.generateNodeID()
			fileType := s.getFileType(file.Name())
			node := RelevantFileNode{
				ID:         nodeID,
				Name:       file.Name(),
				Path:       relPath,
				Type:       "file",
				FileType:   fileType,
				Size:       file.Size(),
				Selectable: true,
			}

			parent.Children = append(parent.Children, node)
		}
	}

	return nil
}

// isRelevantFile 检查是否是相关文件
func (s *RelevantFileScanner) isRelevantFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	lower := strings.ToLower(name)

	// 检查特殊文件
	if lower == "survey.yml" || lower == "survey.yaml" || lower == "survey.json" {
		return true
	}

	// 检查扩展名
	switch ext {
	case ".yml", ".yaml", ".j2", ".sh":
		return true
	default:
		return false
	}
}

// getFileType 获取文件类型
func (s *RelevantFileScanner) getFileType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	lower := strings.ToLower(name)

	// survey文件
	if lower == "survey.yml" || lower == "survey.yaml" || lower == "survey.json" {
		return "survey"
	}

	// 根据扩展名判断
	switch ext {
	case ".yml", ".yaml":
		return "ansible"
	case ".j2":
		return "template"
	case ".sh":
		return "shell"
	default:
		return "unknown"
	}
}

// shouldIgnore 检查是否应该忽略文件或目录
func (s *RelevantFileScanner) shouldIgnore(name string) bool {
	// 隐藏文件（除了某些特殊情况）
	if strings.HasPrefix(name, ".") && name != ".awx" {
		return true
	}

	// 检查忽略模式
	for _, pattern := range s.ignorePatterns {
		if name == pattern {
			return true
		}
	}

	return false
}

// generateNodeID 生成节点ID
func (s *RelevantFileScanner) generateNodeID() string {
	s.nodeIDCounter++
	return fmt.Sprintf("node_%d", s.nodeIDCounter)
}

// removeEmptyDirectories 递归删除空目录
func (s *RelevantFileScanner) removeEmptyDirectories(node *RelevantFileNode) bool {
	if node.Type == "file" {
		return false // 文件不为空
	}

	// 递归处理子节点
	newChildren := []RelevantFileNode{}
	for i := range node.Children {
		if !s.removeEmptyDirectories(&node.Children[i]) {
			newChildren = append(newChildren, node.Children[i])
		}
	}
	node.Children = newChildren

	// 如果目录没有子节点，则认为是空的
	return len(node.Children) == 0
}