package scanner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// SurveyInfo Survey文件信息
type SurveyInfo struct {
	Path        string             `json:"path"`        // survey文件相对路径
	Name        string             `json:"name"`        // survey名称
	Description string             `json:"description"` // survey描述
	Parameters  []SurveyParameter  `json:"parameters"`  // 参数列表
}

// SurveyParameter Survey参数定义
type SurveyParameter struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	MinValue    *string  `json:"min_value,omitempty"`
	MaxValue    *string  `json:"max_value,omitempty"`
	MinLength   *int     `json:"min_length,omitempty"`
	MaxLength   *int     `json:"max_length,omitempty"`
	Source      string   `json:"source,omitempty"`
}

// SurveyScanner Survey文件扫描器
type SurveyScanner struct {
	log *logrus.Logger
}

// NewSurveyScanner 创建Survey扫描器
func NewSurveyScanner(log *logrus.Logger) *SurveyScanner {
	return &SurveyScanner{
		log: log,
	}
}

// ScanSurveys 扫描目录中的所有survey文件
func (s *SurveyScanner) ScanSurveys(projectPath string) ([]SurveyInfo, error) {
	s.log.WithField("path", projectPath).Info("开始扫描survey文件")

	var surveys []SurveyInfo

	// 递归查找所有survey文件
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			s.log.WithError(err).WithField("path", path).Warn("访问文件失败")
			return nil // 继续扫描
		}

		// 跳过目录
		if info.IsDir() {
			// 跳过隐藏目录和常见的忽略目录
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否是survey文件
		if !isSurveyFile(info.Name()) {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(projectPath, path)
		if err != nil {
			s.log.WithError(err).WithField("path", path).Warn("计算相对路径失败")
			return nil
		}

		// 解析survey文件
		surveyInfo, err := s.parseSurveyFile(path, relPath)
		if err != nil {
			s.log.WithError(err).WithField("path", path).Warn("解析survey文件失败")
			return nil
		}

		if surveyInfo != nil {
			surveys = append(surveys, *surveyInfo)
			s.log.WithFields(logrus.Fields{
				"path":       relPath,
				"name":       surveyInfo.Name,
				"parameters": len(surveyInfo.Parameters),
			}).Info("找到survey文件")
		}

		return nil
	})

	if err != nil {
		s.log.WithError(err).Error("扫描survey文件失败")
		return nil, err
	}

	s.log.WithField("count", len(surveys)).Info("survey文件扫描完成")
	return surveys, nil
}

// parseSurveyFile 解析survey文件
func (s *SurveyScanner) parseSurveyFile(filePath, relPath string) (*SurveyInfo, error) {
	// 读取文件内容
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// 根据文件扩展名选择解析方式
	ext := strings.ToLower(filepath.Ext(filePath))
	
	var survey Survey
	if ext == ".json" {
		err = json.Unmarshal(content, &survey)
	} else {
		err = yaml.Unmarshal(content, &survey)
	}

	if err != nil {
		return nil, err
	}

	// 构建SurveyInfo
	info := &SurveyInfo{
		Path:        relPath,
		Name:        survey.Name,
		Description: survey.Description,
		Parameters:  []SurveyParameter{},
	}

	// 转换参数格式
	for _, spec := range survey.Spec {
		param := SurveyParameter{
			Name:        spec.Variable,
			Type:        mapSurveyTypeToParamType(spec.Type),
			Description: spec.QuestionName,
			Required:    spec.Required,
			Options:     spec.Choices,
			Source:      "survey",
		}
		
		// 处理默认值
		if spec.Default != nil {
			param.Default = fmt.Sprintf("%v", spec.Default)
		}

		// 处理验证规则
		if spec.Min != nil {
			minStr := fmt.Sprintf("%d", *spec.Min)
			param.MinValue = &minStr
			
			// 对于字符串类型，Min/Max代表长度
			if spec.Type == "text" || spec.Type == "textarea" || spec.Type == "password" {
				param.MinLength = spec.Min
			}
		}

		if spec.Max != nil {
			maxStr := fmt.Sprintf("%d", *spec.Max)
			param.MaxValue = &maxStr
			
			// 对于字符串类型，Min/Max代表长度
			if spec.Type == "text" || spec.Type == "textarea" || spec.Type == "password" {
				param.MaxLength = spec.Max
			}
		}

		info.Parameters = append(info.Parameters, param)
	}

	return info, nil
}

// mapSurveyTypeToParamType 映射survey类型到参数类型
func mapSurveyTypeToParamType(surveyType string) string {
	switch surveyType {
	case "text":
		return "string"
	case "textarea":
		return "text"
	case "integer":
		return "number"
	case "float":
		return "number"
	case "multiplechoice":
		return "select"
	case "multiselect":
		return "multiselect"
	case "boolean":
		return "boolean"
	case "password":
		return "password"
	default:
		return "string"
	}
}