package services

import (
	"ahop/internal/models"
	"encoding/json"
	"errors"
	"fmt"
)

// WorkflowParser 工作流解析器
type WorkflowParser struct{}

// NewWorkflowParser 创建工作流解析器
func NewWorkflowParser() *WorkflowParser {
	return &WorkflowParser{}
}

// ParsedWorkflow 解析后的工作流
type ParsedWorkflow struct {
	Definition  *models.WorkflowDefinition
	NodeMap     map[string]*models.WorkflowNode
	StartNode   *models.WorkflowNode
	EndNodes    []*models.WorkflowNode
	NodeOrder   []string // 拓扑排序后的节点执行顺序
}

// Parse 解析工作流定义
func (p *WorkflowParser) Parse(definitionJSON json.RawMessage) (*ParsedWorkflow, error) {
	// 反序列化工作流定义
	var definition models.WorkflowDefinition
	if err := json.Unmarshal(definitionJSON, &definition); err != nil {
		return nil, fmt.Errorf("解析工作流定义失败: %v", err)
	}

	// 构建节点映射
	nodeMap := make(map[string]*models.WorkflowNode)
	for i := range definition.Nodes {
		node := &definition.Nodes[i]
		if _, exists := nodeMap[node.ID]; exists {
			return nil, fmt.Errorf("节点ID重复: %s", node.ID)
		}
		
		
		nodeMap[node.ID] = node
	}

	// 查找开始节点
	var startNode *models.WorkflowNode
	for _, node := range nodeMap {
		if node.Type == models.NodeTypeStart {
			if startNode != nil {
				return nil, errors.New("工作流只能有一个开始节点")
			}
			startNode = node
		}
	}
	if startNode == nil {
		return nil, errors.New("工作流必须有开始节点")
	}

	// 查找结束节点
	var endNodes []*models.WorkflowNode
	for _, node := range nodeMap {
		if node.Type == models.NodeTypeEnd {
			endNodes = append(endNodes, node)
		}
	}
	if len(endNodes) == 0 {
		return nil, errors.New("工作流必须有至少一个结束节点")
	}

	// 验证节点连接
	if err := p.validateConnections(&definition, nodeMap); err != nil {
		return nil, err
	}

	// 拓扑排序
	nodeOrder, err := p.topologicalSort(nodeMap, startNode)
	if err != nil {
		return nil, err
	}

	return &ParsedWorkflow{
		Definition: &definition,
		NodeMap:    nodeMap,
		StartNode:  startNode,
		EndNodes:   endNodes,
		NodeOrder:  nodeOrder,
	}, nil
}

// validateConnections 验证节点连接
func (p *WorkflowParser) validateConnections(def *models.WorkflowDefinition, nodeMap map[string]*models.WorkflowNode) error {
	// 验证连接中的节点是否存在
	for _, conn := range def.Connections {
		if _, exists := nodeMap[conn.From]; !exists {
			return fmt.Errorf("连接的源节点不存在: %s", conn.From)
		}
		if _, exists := nodeMap[conn.To]; !exists {
			return fmt.Errorf("连接的目标节点不存在: %s", conn.To)
		}
	}

	// 验证节点的next_nodes是否有效
	for _, node := range nodeMap {
		for _, nextID := range node.NextNodes {
			if _, exists := nodeMap[nextID]; !exists {
				return fmt.Errorf("节点 %s 的下一节点 %s 不存在", node.ID, nextID)
			}
		}
	}

	// 验证条件节点
	for _, node := range nodeMap {
		if node.Type == models.NodeTypeCondition {
			if node.Config == nil {
				return fmt.Errorf("条件节点 %s 缺少配置", node.ID)
			}
			
			// 条件节点必须有且仅有2个next_nodes
			if len(node.NextNodes) != 2 {
				return fmt.Errorf("条件节点 %s 必须有且仅有2个next_nodes（[true分支, false分支]）", node.ID)
			}
			
			// 验证 next_nodes 中的节点是否存在
			if _, exists := nodeMap[node.NextNodes[0]]; !exists {
				return fmt.Errorf("条件节点 %s 的 true 分支节点 %s 不存在", node.ID, node.NextNodes[0])
			}
			if _, exists := nodeMap[node.NextNodes[1]]; !exists {
				return fmt.Errorf("条件节点 %s 的 false 分支节点 %s 不存在", node.ID, node.NextNodes[1])
			}
		}
	}

	return nil
}

// topologicalSort 拓扑排序
func (p *WorkflowParser) topologicalSort(nodeMap map[string]*models.WorkflowNode, startNode *models.WorkflowNode) ([]string, error) {
	// 构建邻接表
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	
	// 初始化
	for id := range nodeMap {
		graph[id] = []string{}
		inDegree[id] = 0
	}
	
	// 构建图
	for _, node := range nodeMap {
		for _, nextID := range node.NextNodes {
			graph[node.ID] = append(graph[node.ID], nextID)
			inDegree[nextID]++
		}
		
	}
	
	// 拓扑排序
	var result []string
	queue := []string{startNode.ID}
	
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		for _, next := range graph[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	
	// 检查是否有环
	if len(result) != len(nodeMap) {
		return nil, errors.New("工作流中存在循环依赖")
	}
	
	return result, nil
}

// GetNextNodes 获取节点的下一个执行节点
func (p *WorkflowParser) GetNextNodes(node *models.WorkflowNode, context map[string]interface{}) []string {
	if node.Type == models.NodeTypeCondition {
		// 条件节点根据条件判断下一个节点
		// 使用 next_nodes: [true分支, false分支]
		condition, ok := evaluateCondition(node, context)
		if !ok {
			return []string{} // 条件评估失败，不继续执行
		}
		
		if condition {
			return []string{node.NextNodes[0]} // true分支
		} else {
			return []string{node.NextNodes[1]} // false分支
		}
	}
	
	// 其他节点直接返回next_nodes
	return node.NextNodes
}

// evaluateCondition 评估条件（简化版本）
func evaluateCondition(node *models.WorkflowNode, context map[string]interface{}) (bool, bool) {
	if node.Config == nil {
		return false, false
	}
	
	_, ok := node.Config["expression"].(string)
	if !ok {
		return false, false
	}
	
	// TODO: 实现表达式评估逻辑
	// 这里是简化版本，实际需要实现表达式解析器
	// 例如: "task_result.status == 'success'"
	
	// 暂时返回true，后续实现
	return true, true
}

// contains 检查字符串切片是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}