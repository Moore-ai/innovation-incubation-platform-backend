package service

import (
	"context"
	"io"

	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/fileparser"
)

type TestService struct {
	llm       *aiclient.Client
	embedding *aiclient.EmbeddingClient
}

func NewTestService(llm *aiclient.Client, embedding *aiclient.EmbeddingClient) *TestService {
	return &TestService{llm: llm, embedding: embedding}
}

// TestLLM 发送简单 prompt 验证 LLM 可用性
func (s *TestService) TestLLM(ctx context.Context) (string, error) {
	return s.llm.Chat(ctx, "你是一个测试助手", "请回复：LLM 连接正常。")
}

// TestEmbedding 将一段文本向量化验证 Embedding 模型可用性
func (s *TestService) TestEmbedding(ctx context.Context) (int, error) {
	embedding, err := s.embedding.Embed(ctx, "这是一个测试文本，用于验证嵌入模型是否正常工作。")
	if err != nil {
		return 0, err
	}
	return len(embedding), nil
}

// IsLLMAvailable 检查 LLM client 是否已初始化
func (s *TestService) IsLLMAvailable() bool {
	return s.llm != nil
}

// IsEmbeddingAvailable 检查 Embedding client 是否已初始化
func (s *TestService) IsEmbeddingAvailable() bool {
	return s.embedding != nil
}

// TestConvertFile 调用 fileparser 将文件转为 markdown
func (s *TestService) TestConvertFile(r io.Reader, size int64, ext string) (string, error) {
	return fileparser.Parse(r, size, ext)
}
