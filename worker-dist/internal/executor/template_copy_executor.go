package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// TemplateCopyExecutor 模板文件复制执行器
type TemplateCopyExecutor struct {
	repoBaseDir     string
	templateBaseDir string
	log             *logrus.Logger
}

// NewTemplateCopyExecutor 创建模板文件复制执行器
func NewTemplateCopyExecutor(repoBaseDir, templateBaseDir string, log *logrus.Logger) *TemplateCopyExecutor {
	return &TemplateCopyExecutor{
		repoBaseDir:     repoBaseDir,
		templateBaseDir: templateBaseDir,
		log:             log,
	}
}

// TemplateCopyMessage 模板复制消息
type TemplateCopyMessage struct {
	Action        string      `json:"action"` // copy/delete
	TemplateID    uint        `json:"template_id"`
	TenantID      uint        `json:"tenant_id"`
	TemplateCode  string      `json:"template_code"`
	RepositoryID  uint        `json:"repository_id"`
	SourcePath    string      `json:"source_path"`    // Git仓库中的原始路径
	EntryFile     string      `json:"entry_file"`     // 相对于模板目录的入口文件
	IncludedFiles []IncludedFile `json:"included_files"` // 包含的文件列表
}

// IncludedFile 包含的文件
type IncludedFile struct {
	Path    string   `json:"path"`     // 相对路径
	Type    string   `json:"type"`     // file/directory
	Pattern string   `json:"pattern"`  // 当type为pattern时的模式
}

// HandleTemplateCopy 处理模板复制消息
func (e *TemplateCopyExecutor) HandleTemplateCopy(ctx context.Context, message []byte) error {
	var msg TemplateCopyMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return fmt.Errorf("解析消息失败: %v", err)
	}

	// 打印原始消息内容用于调试
	e.log.WithField("raw_message", string(message)).Debug("收到原始模板复制消息")

	e.log.WithFields(logrus.Fields{
		"action":          msg.Action,
		"template_id":     msg.TemplateID,
		"tenant_id":       msg.TenantID,
		"template_code":    msg.TemplateCode,
		"entry_file":      msg.EntryFile,
		"included_files":  len(msg.IncludedFiles),
	}).Info("处理模板文件操作")

	switch msg.Action {
	case "copy":
		return e.copyTemplateFiles(ctx, &msg)
	case "delete":
		return e.deleteTemplateFiles(ctx, &msg)
	default:
		return fmt.Errorf("未知的操作类型: %s", msg.Action)
	}
}

// copyTemplateFiles 复制模板文件到独立目录
func (e *TemplateCopyExecutor) copyTemplateFiles(ctx context.Context, msg *TemplateCopyMessage) error {
	// 构建源路径（Git仓库）
	// 假设Git仓库存储格式为: repos/{tenant_id}/{repo_id}
	repoPath := filepath.Join(e.repoBaseDir, fmt.Sprintf("%d/%d", msg.TenantID, msg.RepositoryID))
	sourcePath := filepath.Join(repoPath, msg.SourcePath)

	// 构建目标路径（独立模板目录）
	targetPath := filepath.Join(e.templateBaseDir, fmt.Sprintf("%d/%s", msg.TenantID, msg.TemplateCode))

	// 确保目标目录存在
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 复制入口文件
	entrySource := filepath.Join(sourcePath, msg.EntryFile)
	entryTarget := filepath.Join(targetPath, msg.EntryFile)
	
	e.log.WithFields(logrus.Fields{
		"entry_source": entrySource,
		"entry_target": entryTarget,
	}).Debug("复制入口文件")
	
	if err := e.copyFile(entrySource, entryTarget); err != nil {
		return fmt.Errorf("复制入口文件失败: %v", err)
	}

	// 复制包含的文件
	e.log.WithField("included_files_count", len(msg.IncludedFiles)).Info("开始复制包含的文件")
	for _, includedFile := range msg.IncludedFiles {
		switch includedFile.Type {
		case "file":
			src := filepath.Join(sourcePath, includedFile.Path)
			dst := filepath.Join(targetPath, includedFile.Path)
			
			e.log.WithFields(logrus.Fields{
				"file_path": includedFile.Path,
				"src":       src,
				"dst":       dst,
			}).Debug("复制包含文件")
			
			// 确保目标文件的目录存在
			dstDir := filepath.Dir(dst)
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return fmt.Errorf("创建目录失败 %s: %v", dstDir, err)
			}
			
			if err := e.copyFile(src, dst); err != nil {
				e.log.WithError(err).Warnf("复制文件失败: %s", includedFile.Path)
				// 继续处理其他文件
			}
			
		case "directory":
			src := filepath.Join(sourcePath, includedFile.Path)
			dst := filepath.Join(targetPath, includedFile.Path)
			
			if err := e.copyDirectory(src, dst); err != nil {
				e.log.WithError(err).Warnf("复制目录失败: %s", includedFile.Path)
				// 继续处理其他文件
			}
			
		case "pattern":
			// 实现模式匹配复制
			matches, err := filepath.Glob(filepath.Join(sourcePath, includedFile.Pattern))
			if err != nil {
				e.log.WithError(err).Warnf("模式匹配失败: %s", includedFile.Pattern)
				continue
			}
			
			for _, matchedFile := range matches {
				// 计算相对路径
				relPath, err := filepath.Rel(sourcePath, matchedFile)
				if err != nil {
					e.log.WithError(err).Warnf("计算相对路径失败: %s", matchedFile)
					continue
				}
				
				// 构建目标路径
				dst := filepath.Join(targetPath, relPath)
				
				// 获取文件信息
				fileInfo, err := os.Stat(matchedFile)
				if err != nil {
					e.log.WithError(err).Warnf("获取文件信息失败: %s", matchedFile)
					continue
				}
				
				if fileInfo.IsDir() {
					// 复制目录
					if err := e.copyDirectory(matchedFile, dst); err != nil {
						e.log.WithError(err).Warnf("复制目录失败: %s", matchedFile)
					}
				} else {
					// 确保目标目录存在
					dstDir := filepath.Dir(dst)
					if err := os.MkdirAll(dstDir, 0755); err != nil {
						e.log.WithError(err).Warnf("创建目录失败: %s", dstDir)
						continue
					}
					
					// 复制文件
					if err := e.copyFile(matchedFile, dst); err != nil {
						e.log.WithError(err).Warnf("复制文件失败: %s", matchedFile)
					}
				}
			}
		}
	}

	e.log.WithFields(logrus.Fields{
		"template_id":   msg.TemplateID,
		"template_code": msg.TemplateCode,
		"target_path":   targetPath,
	}).Info("模板文件复制完成")

	return nil
}

// deleteTemplateFiles 删除模板文件
func (e *TemplateCopyExecutor) deleteTemplateFiles(ctx context.Context, msg *TemplateCopyMessage) error {
	targetPath := filepath.Join(e.templateBaseDir, fmt.Sprintf("%d/%s", msg.TenantID, msg.TemplateCode))

	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("删除模板目录失败: %v", err)
	}

	e.log.WithFields(logrus.Fields{
		"template_id":   msg.TemplateID,
		"template_code": msg.TemplateCode,
		"target_path":   targetPath,
	}).Info("模板文件删除完成")

	return nil
}

// copyFile 复制单个文件
func (e *TemplateCopyExecutor) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %v", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	// 保持文件权限
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("获取源文件信息失败: %v", err)
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

// copyDirectory 递归复制目录
func (e *TemplateCopyExecutor) copyDirectory(src, dst string) error {
	// 获取源目录信息
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("获取源目录信息失败: %v", err)
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 读取源目录内容
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %v", err)
	}

	// 递归复制每个条目
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := e.copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := e.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}