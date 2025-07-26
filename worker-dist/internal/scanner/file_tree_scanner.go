package scanner

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// FileNode 文件节点
type FileNode struct {
	Name     string      `json:"name"`               // 文件或目录名
	Path     string      `json:"path"`               // 相对于仓库根目录的路径
	Type     string      `json:"type"`               // file/directory
	Size     int64       `json:"size"`               // 文件大小（字节）
	Children []FileNode  `json:"children,omitempty"` // 子节点（目录才有）
}

// FileTreeScanner 文件树扫描器
type FileTreeScanner struct {
	log            *logrus.Logger
	ignorePatterns []string
	maxDepth       int
}

// NewFileTreeScanner 创建文件树扫描器
func NewFileTreeScanner(log *logrus.Logger) *FileTreeScanner {
	return &FileTreeScanner{
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
			"*.pyc",
			"*.pyo",
			".DS_Store",
			"Thumbs.db",
		},
		maxDepth: 10, // 最大递归深度
	}
}

// ScanFileTree 扫描文件树
func (s *FileTreeScanner) ScanFileTree(rootPath string) (*FileNode, error) {
	s.log.WithField("path", rootPath).Info("开始扫描文件树")

	// 检查根目录
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, os.ErrInvalid
	}

	// 创建根节点
	root := &FileNode{
		Name:     filepath.Base(rootPath),
		Path:     ".",
		Type:     "directory",
		Size:     0,
		Children: []FileNode{},
	}

	// 递归扫描
	if err := s.scanDirectory(rootPath, rootPath, root, 0); err != nil {
		s.log.WithError(err).Warn("扫描目录时出现错误")
		// 继续返回部分结果
	}

	s.log.WithField("path", rootPath).Info("文件树扫描完成")
	return root, nil
}

// scanDirectory 递归扫描目录
func (s *FileTreeScanner) scanDirectory(rootPath, currentPath string, parent *FileNode, depth int) error {
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

		// 创建节点
		node := FileNode{
			Name: file.Name(),
			Path: relPath,
			Size: file.Size(),
		}

		if file.IsDir() {
			node.Type = "directory"
			node.Children = []FileNode{}

			// 递归扫描子目录
			if err := s.scanDirectory(rootPath, fullPath, &node, depth+1); err != nil {
				s.log.WithError(err).WithField("path", fullPath).Debug("扫描子目录失败")
				// 继续处理其他文件
			}
		} else {
			node.Type = "file"
		}

		parent.Children = append(parent.Children, node)
	}

	return nil
}

// shouldIgnore 检查是否应该忽略文件或目录
func (s *FileTreeScanner) shouldIgnore(name string) bool {
	for _, pattern := range s.ignorePatterns {
		// 精确匹配
		if name == pattern {
			return true
		}

		// 模式匹配
		if strings.HasPrefix(pattern, "*") {
			if strings.HasSuffix(name, strings.TrimPrefix(pattern, "*")) {
				return true
			}
		}
	}
	return false
}

// FindSurveyFiles 在文件树中查找所有survey文件
func FindSurveyFiles(root *FileNode) []string {
	var surveyFiles []string
	findSurveyFilesRecursive(root, &surveyFiles)
	return surveyFiles
}

// findSurveyFilesRecursive 递归查找survey文件
func findSurveyFilesRecursive(node *FileNode, files *[]string) {
	if node.Type == "file" && isSurveyFile(node.Name) {
		*files = append(*files, node.Path)
	}

	for i := range node.Children {
		findSurveyFilesRecursive(&node.Children[i], files)
	}
}

// isSurveyFile 检查是否是survey文件
func isSurveyFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == "survey.yml" || lower == "survey.yaml" || lower == "survey.json"
}