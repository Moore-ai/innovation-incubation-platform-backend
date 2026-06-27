package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type TestController struct {
	svc *service.TestService
}

func NewTestController(svc *service.TestService) *TestController {
	return &TestController{svc: svc}
}

// TestLLM 发送简单 prompt 验证 LLM 可用性
func (ctrl *TestController) TestLLM(c *gin.Context) {
	if !ctrl.svc.IsLLMAvailable() {
		response.Error(c, errcode.ErrAIService.WithMsg("LLM client 未初始化（检查 AI_API_KEY 配置）"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	reply, err := ctrl.svc.TestLLM(ctx)
	if err != nil {
		response.Error(c, errcode.ErrAIService.WithMsg(fmt.Sprintf("LLM 调用失败: %v", err)))
		return
	}

	response.Success(c, gin.H{
		"status":  "ok",
		"message": "LLM 连接正常",
		"reply":   reply,
	})
}

// TestEmbedding 将一段文本向量化验证 Embedding 模型可用性
func (ctrl *TestController) TestEmbedding(c *gin.Context) {
	if !ctrl.svc.IsEmbeddingAvailable() {
		response.Error(c, errcode.ErrAIService.WithMsg("Embedding client 未初始化（检查 EMBEDDING_API_KEY 配置）"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	dimension, err := ctrl.svc.TestEmbedding(ctx)
	if err != nil {
		response.Error(c, errcode.ErrAIService.WithMsg(fmt.Sprintf("Embedding 调用失败: %v", err)))
		return
	}

	response.Success(c, gin.H{
		"status":    "ok",
		"message":   "Embedding 连接正常",
		"dimension": dimension,
	})
}

// TestConvertFile 上传文件，调用 markitdown 转换为 markdown 并返回
func (ctrl *TestController) TestConvertFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("请上传文件（form-data, field: file）"))
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("文件缺少扩展名"))
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(fmt.Sprintf("读取文件失败: %v", err)))
		return
	}

	markdown, err := ctrl.svc.TestConvertFile(bytes.NewReader(data), int64(len(data)), ext)
	if err != nil {
		response.Error(c, errcode.ErrAIService.WithMsg(fmt.Sprintf("文件转换失败: %v", err)))
		return
	}

	response.Success(c, gin.H{
		"status":   "ok",
		"filename": header.Filename,
		"ext":      ext,
		"markdown": markdown,
	})
}
