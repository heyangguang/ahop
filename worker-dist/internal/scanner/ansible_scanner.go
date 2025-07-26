package scanner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// AnsibleScanner Ansible 项目扫描器
type AnsibleScanner struct {
	log *logrus.Logger
}

// NewAnsibleScanner 创建 Ansible 扫描器
func NewAnsibleScanner(log *logrus.Logger) *AnsibleScanner {
	return &AnsibleScanner{
		log: log,
	}
}

// Scan 扫描 Ansible 项目
func (s *AnsibleScanner) Scan(projectPath string) ([]ScanResult, error) {
	var results []ScanResult
	
	// 查找 survey 文件的位置
	surveyPaths := []string{
		filepath.Join(projectPath, "survey.json"),
		filepath.Join(projectPath, "survey.yml"),
		filepath.Join(projectPath, "survey.yaml"),
		filepath.Join(projectPath, ".awx", "survey.json"),
		filepath.Join(projectPath, ".awx", "survey.yml"),
		filepath.Join(projectPath, ".awx", "survey.yaml"),
	}
	
	for _, surveyPath := range surveyPaths {
		if _, err := os.Stat(surveyPath); err == nil {
			s.log.WithField("path", surveyPath).Debug("找到 survey 文件")
			
			// 解析 survey 文件
			survey, err := s.parseSurveyFile(surveyPath)
			if err != nil {
				s.log.WithError(err).WithField("path", surveyPath).Warn("解析 survey 文件失败")
				continue
			}
			
			// 查找对应的 playbook
			playbookPath := s.findPlaybook(projectPath, surveyPath)
			if playbookPath == "" {
				s.log.WithField("survey", surveyPath).Warn("未找到对应的 playbook")
				continue
			}
			
			results = append(results, ScanResult{
				Type:   "ansible",
				Name:   survey.Name,
				Path:   playbookPath,
				Survey: survey,
			})
		}
	}
	
	// 扫描 surveys 目录（支持多个 survey）
	surveysDir := filepath.Join(projectPath, "surveys")
	if info, err := os.Stat(surveysDir); err == nil && info.IsDir() {
		files, _ := ioutil.ReadDir(surveysDir)
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".json") || 
			   strings.HasSuffix(file.Name(), ".yml") || 
			   strings.HasSuffix(file.Name(), ".yaml") {
				surveyPath := filepath.Join(surveysDir, file.Name())
				survey, err := s.parseSurveyFile(surveyPath)
				if err != nil {
					s.log.WithError(err).WithField("path", surveyPath).Warn("解析 survey 文件失败")
					continue
				}
				
				// 基于文件名查找 playbook
				playbookName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
				playbookPath := s.findPlaybookByName(projectPath, playbookName)
				if playbookPath != "" {
					results = append(results, ScanResult{
						Type:   "ansible",
						Name:   survey.Name,
						Path:   playbookPath,
						Survey: survey,
					})
				}
			}
		}
	}
	
	return results, nil
}

// parseSurveyFile 解析 survey 文件
func (s *AnsibleScanner) parseSurveyFile(path string) (*Survey, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var survey Survey
	
	// 根据文件扩展名选择解析方式
	if strings.HasSuffix(path, ".json") {
		err = json.Unmarshal(data, &survey)
	} else {
		err = yaml.Unmarshal(data, &survey)
	}
	
	if err != nil {
		return nil, err
	}
	
	// 验证必填字段
	if survey.Name == "" {
		return nil, fmt.Errorf("survey name is required")
	}
	
	// 验证 spec
	if len(survey.Spec) == 0 {
		return nil, fmt.Errorf("survey spec is empty")
	}
	
	// 验证每个 item
	for i, item := range survey.Spec {
		if item.Variable == "" {
			return nil, fmt.Errorf("survey spec[%d]: variable is required", i)
		}
		if item.Type == "" {
			survey.Spec[i].Type = "text" // 默认类型
		}
		if item.QuestionName == "" {
			survey.Spec[i].QuestionName = item.Variable // 使用变量名作为默认问题
		}
	}
	
	return &survey, nil
}

// findPlaybook 查找对应的 playbook
func (s *AnsibleScanner) findPlaybook(projectPath, surveyPath string) string {
	// 优先级顺序查找
	candidates := []string{
		filepath.Join(projectPath, "playbook.yml"),
		filepath.Join(projectPath, "playbook.yaml"),
		filepath.Join(projectPath, "site.yml"),
		filepath.Join(projectPath, "site.yaml"),
		filepath.Join(projectPath, "main.yml"),
		filepath.Join(projectPath, "main.yaml"),
		filepath.Join(projectPath, "deploy.yml"),
		filepath.Join(projectPath, "deploy.yaml"),
	}
	
	// 如果有 playbooks 目录，查找其中的第一个 .yml 文件
	playbooksDir := filepath.Join(projectPath, "playbooks")
	if info, err := os.Stat(playbooksDir); err == nil && info.IsDir() {
		files, _ := ioutil.ReadDir(playbooksDir)
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".yaml") {
				candidate := filepath.Join(playbooksDir, file.Name())
				s.log.WithField("playbook", candidate).Debug("在 playbooks 目录找到 playbook")
				return candidate
			}
		}
	}
	
	// 如果 survey 在子目录，也在同目录查找
	surveyDir := filepath.Dir(surveyPath)
	if surveyDir != projectPath {
		candidates = append([]string{
			filepath.Join(surveyDir, "playbook.yml"),
			filepath.Join(surveyDir, "playbook.yaml"),
			filepath.Join(surveyDir, "main.yml"),
			filepath.Join(surveyDir, "main.yaml"),
		}, candidates...)
	}
	
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			s.log.WithField("playbook", candidate).Debug("找到对应的 playbook")
			return candidate
		}
	}
	
	return ""
}

// findPlaybookByName 根据名称查找 playbook
func (s *AnsibleScanner) findPlaybookByName(projectPath, name string) string {
	candidates := []string{
		filepath.Join(projectPath, name+".yml"),
		filepath.Join(projectPath, name+".yaml"),
		filepath.Join(projectPath, "playbooks", name+".yml"),
		filepath.Join(projectPath, "playbooks", name+".yaml"),
		filepath.Join(projectPath, "ansible", name+".yml"),
		filepath.Join(projectPath, "ansible", name+".yaml"),
	}
	
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			s.log.WithField("playbook", candidate).Debug("根据名称找到 playbook")
			return candidate
		}
	}
	
	return ""
}