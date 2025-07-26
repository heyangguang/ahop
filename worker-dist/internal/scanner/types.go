package scanner

// ScanResult 扫描结果
type ScanResult struct {
	Type      string  `json:"type"`      // ansible|shell
	Name      string  `json:"name"`      // 模板名称
	Path      string  `json:"path"`      // 脚本路径
	Survey    *Survey `json:"survey"`    // Survey 定义
}

// Survey AWX 标准 Survey 结构
type Survey struct {
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description" yaml:"description"`
	Spec        []SurveyItem `json:"spec" yaml:"spec"`
}

// SurveyItem Survey 项目定义
type SurveyItem struct {
	Type                string      `json:"type" yaml:"type"`                                           // text|textarea|password|integer|float|multiplechoice|multiselect
	Variable            string      `json:"variable" yaml:"variable"`                                   // 变量名
	QuestionName        string      `json:"question_name" yaml:"question_name"`                        // 问题标题
	QuestionDescription string      `json:"question_description,omitempty" yaml:"question_description"` // 问题描述
	Required            bool        `json:"required" yaml:"required"`                                   // 是否必填
	Default             interface{} `json:"default,omitempty" yaml:"default"`                          // 默认值
	Choices             []string    `json:"choices,omitempty" yaml:"choices"`                          // 选项列表
	Min                 *int        `json:"min,omitempty" yaml:"min"`                                  // 最小值/长度
	Max                 *int        `json:"max,omitempty" yaml:"max"`                                  // 最大值/长度
}