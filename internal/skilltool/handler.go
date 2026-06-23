package skilltool

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/llm"
)

type ToolHandler struct {
	executor *Executor
	cfg      GoJudgeConfig
}

type GoJudgeConfig struct {
	Endpoint string
}

func NewToolHandler(endpoint string) *ToolHandler {
	return &ToolHandler{
		executor: NewExecutor(endpoint),
		cfg:      GoJudgeConfig{Endpoint: endpoint},
	}
}

func (h *ToolHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("", h.listTools)
	r.POST("", h.createTool)
	r.GET("/:toolId", h.getTool)
	r.PUT("/:toolId", h.updateTool)
	r.DELETE("/:toolId", h.deleteTool)
	r.POST("/:toolId/polish", h.polishTool)
	r.POST("/:toolId/debug", h.debugTool)
}

func (h *ToolHandler) listTools(c *gin.Context) {
	botID := c.Param("id")
	skillID, _ := strconv.ParseUint(c.Param("skillId"), 10, 64)
	tools, err := ListToolsBySkill(botID, uint(skillID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tools)
}

func (h *ToolHandler) getTool(c *gin.Context) {
	botID := c.Param("id")
	skillID, _ := strconv.ParseUint(c.Param("skillId"), 10, 64)
	toolID, _ := strconv.ParseUint(c.Param("toolId"), 10, 64)
	tool, err := GetToolByID(botID, uint(skillID), uint(toolID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool 未找到"})
		return
	}
	c.JSON(http.StatusOK, tool)
}

func (h *ToolHandler) createTool(c *gin.Context) {
	botID := c.Param("id")
	skillID, _ := strconv.ParseUint(c.Param("skillId"), 10, 64)

	var req struct {
		Name         string `json:"name" binding:"required"`
		Language     string `json:"language" binding:"required"`
		Code         string `json:"code"`
		InputParams  string `json:"input_params"`
		OutputParams string `json:"output_params"`
		Prompt       string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	tool := db.GoJudgeTool{
		BotID:        botID,
		SkillID:      uint(skillID),
		Name:         req.Name,
		Language:     req.Language,
		Code:         req.Code,
		InputParams:  req.InputParams,
		OutputParams: req.OutputParams,
		Prompt:       req.Prompt,
	}
	if err := CreateTool(&tool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tool)
}

func (h *ToolHandler) updateTool(c *gin.Context) {
	botID := c.Param("id")
	skillID, _ := strconv.ParseUint(c.Param("skillId"), 10, 64)
	toolID, _ := strconv.ParseUint(c.Param("toolId"), 10, 64)

	var req struct {
		Name         *string `json:"name"`
		Language     *string `json:"language"`
		Code         *string `json:"code"`
		InputParams  *string `json:"input_params"`
		OutputParams *string `json:"output_params"`
		Prompt       *string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Language != nil {
		updates["language"] = *req.Language
	}
	if req.Code != nil {
		updates["code"] = *req.Code
	}
	if req.InputParams != nil {
		updates["input_params"] = *req.InputParams
	}
	if req.OutputParams != nil {
		updates["output_params"] = *req.OutputParams
	}
	if req.Prompt != nil {
		updates["prompt"] = *req.Prompt
	}

	if err := UpdateTool(botID, uint(skillID), uint(toolID), updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	tool, _ := GetToolByID(botID, uint(skillID), uint(toolID))
	c.JSON(http.StatusOK, tool)
}

func (h *ToolHandler) deleteTool(c *gin.Context) {
	botID := c.Param("id")
	skillID, _ := strconv.ParseUint(c.Param("skillId"), 10, 64)
	toolID, _ := strconv.ParseUint(c.Param("toolId"), 10, 64)

	if err := DeleteTool(botID, uint(skillID), uint(toolID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *ToolHandler) polishTool(c *gin.Context) {
	botID := c.Param("id")
	skillID, _ := strconv.ParseUint(c.Param("skillId"), 10, 64)
	toolID, _ := strconv.ParseUint(c.Param("toolId"), 10, 64)

	tool, err := GetToolByID(botID, uint(skillID), uint(toolID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool 未找到"})
		return
	}

	if tool.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先输入想法"})
		return
	}

	var bot db.Bot
	if err := db.DB.Where("id = ?", botID).First(&bot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "机器人未找到"})
		return
	}

	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", bot.LLMProviderID).First(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM Provider 未找到"})
		return
	}

	langDisplay := tool.Language
	if langDisplay == "" {
		langDisplay = "python3"
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的代码生成助手。根据用户的想法生成 %s 语言的可执行代码。

要求：
1. 代码必须是完整、可执行的，不要包含任何额外的说明文字
2. 如果需要输入参数，使用 stdin 读取（input() 或类似方式）
3. 如果需要输出结果，使用 stdout 打印
4. 代码应当健壮，包含基本的错误处理
5. 不要使用外部库（仅使用标准库）
6. 代码长度控制在 500 行以内`, langDisplay)

	userPrompt := fmt.Sprintf(`请生成 %s 语言的代码，实现以下功能：

%s

请只返回代码，不要包含任何其他说明或 markdown 格式。`, langDisplay, tool.Prompt)

	llmClient := llm.NewOpenAIClient(provider.Endpoint, provider.APIKey, bot.LLMModel)
	llmClient.Temperature = 0.3
	if bot.LLMMaxTokens != nil && *bot.LLMMaxTokens > 0 {
		llmClient.MaxTokens = *bot.LLMMaxTokens
	}

	resp, err := llmClient.Chat([]llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM 调用失败: " + err.Error()})
		return
	}

	generatedCode := resp.Content

	if err := UpdateTool(botID, uint(skillID), uint(toolID), map[string]interface{}{
		"code": generatedCode,
	}); err != nil {
		log.Printf("保存润色后的代码失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":   generatedCode,
		"prompt": tool.Prompt,
	})
}

func (h *ToolHandler) debugTool(c *gin.Context) {
	botID := c.Param("id")
	skillID, _ := strconv.ParseUint(c.Param("skillId"), 10, 64)
	toolID, _ := strconv.ParseUint(c.Param("toolId"), 10, 64)

	tool, err := GetToolByID(botID, uint(skillID), uint(toolID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool 未找到"})
		return
	}

	if tool.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先编写代码"})
		return
	}

	result, err := h.executor.Execute(&ExecuteRequest{
		Lang: tool.Language,
		Src:  tool.Code,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "执行失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
