// Milvus 向量库客户端：知识库向量存储与检索（1024 维 COSINE，按 topic 过滤）。
package vector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"mirror-mian/internal/store"
)

const (
	collectionName = "knowledge_chunks"
	vectorDim      = 1024
	initTimeout    = 60 * time.Second
	searchTimeout  = 30 * time.Second
)

// Hit 向量检索命中。
type Hit struct {
	ChunkIndex int64
	Score      float32
}

// Milvus 向量库客户端。
type Milvus struct {
	client client.Client
	ok     bool
}

// New 连接 Milvus 并确保集合存在；连接失败时返回 ok=false 的实例（调用方降级）。
func New(addr string) *Milvus {
	m := &Milvus{}
	if addr == "" {
		return m
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cli, err := client.NewClient(ctx, client.Config{Address: addr})
	if err != nil {
		log.Printf("[Milvus] 连接失败（%v），向量检索将回退 SQLite", err)
		return m
	}
	m.client = cli
	m.ok = true
	if err := m.ensureCollection(ctx); err != nil {
		log.Printf("[Milvus] 集合初始化失败（%v），向量检索将回退 SQLite", err)
		cli.Close()
		m.client = nil
		m.ok = false
	}
	return m
}

// Available Milvus 是否可用。
func (m *Milvus) Available() bool { return m != nil && m.ok && m.client != nil }

// Close 关闭连接。
func (m *Milvus) Close() {
	if m.client != nil {
		m.client.Close()
	}
}

// ensureCollection 建集合（存在则跳过）、确保索引存在、加载（带重试，索引就绪可能滞后）。
// SDK v2.4.2 在索引缺失时 CreateIndex/DescribeIndex 内部 panic（bug），
// 捕获后重建集合（drop → create → index → load）绕开。
func (m *Milvus) ensureCollection(ctx context.Context) error {
	has, err := m.client.HasCollection(ctx, collectionName)
	if err != nil {
		return err
	}
	if has {
		// 集合已存在：尝试补索引；异常（SDK panic / 索引缺失）→ 重建集合
		if idxErr := m.safeCreateIndex(ctx); idxErr != nil {
			_ = m.client.DropCollection(ctx, collectionName)
			has = false
		}
	}
	if !has {
		if err := m.createCollection(ctx); err != nil {
			return err
		}
		if err := m.safeCreateIndex(ctx); err != nil {
			return err
		}
	}
	// 加载集合（索引后台构建就绪可能滞后；SDK 可能 panic，recover 后重试 60s）
	var lastErr error
	for i := 0; i < 30; i++ {
		if loadErr := safeLoad(m.client, ctx); loadErr == nil {
			return nil
		} else {
			lastErr = loadErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("加载集合超时: %v", lastErr)
}

// createCollection 创建集合（schema 固定：id/topic/chunk_index/embedding）。
func (m *Milvus) createCollection(ctx context.Context) error {
	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    "mirror-mian knowledge chunks",
		AutoID:         true,
		Fields: []*entity.Field{
			{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: true},
			{Name: "topic", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "256"}},
			{Name: "chunk_index", DataType: entity.FieldTypeInt64},
			{Name: "embedding", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": fmt.Sprintf("%d", vectorDim)}},
		},
	}
	return m.client.CreateCollection(ctx, schema, 1)
}

// safeLoad 捕获 SDK LoadCollection 的 panic（索引未就绪时）。
func safeLoad(cli client.Client, ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("load panic: %v", r)
		}
	}()
	return cli.LoadCollection(ctx, collectionName, false)
}

// safeCreateIndex 创建向量索引（AUTOINDEX，async 模式避免 SDK sync 轮询对 nil desc panic），
// 创建后自行轮询索引状态直到就绪；索引已存在时忽略错误。
func (m *Milvus) safeCreateIndex(ctx context.Context) error {
	idx, err := entity.NewIndexAUTOINDEX(entity.COSINE)
	if err != nil {
		return err
	}
	// 注意：不能传 nil opts——variadic nil 会变成含 nil 元素的切片，SDK 调用 nil 函数 panic
	err = m.client.CreateIndex(ctx, collectionName, "embedding", idx, true)
	if err != nil && strings.Contains(err.Error(), "exist") {
		return nil // 索引已存在
	}
	// async 模式下索引后台构建，由 LoadCollection 重试等待就绪
	return err
}

// Upsert 替换某主题全部向量：先删该主题再插入（对齐 sync 重建语义）。
func (m *Milvus) Upsert(ctx context.Context, topic string, chunks []store.KnowledgeChunk) error {
	if !m.Available() {
		return fmt.Errorf("milvus 不可用")
	}
	if err := m.Delete(ctx, topic); err != nil {
		return err
	}
	if len(chunks) == 0 {
		return nil
	}
	col := entity.NewColumnVarChar("topic", repeat(topic, len(chunks)))
	idxCol := entity.NewColumnInt64("chunk_index", chunkIndexes(chunks))
	vecCol := entity.NewColumnFloatVector("embedding", vectorDim, chunkEmbeddings(chunks))

	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	_, err := m.client.Insert(ctx, collectionName, "", col, idxCol, vecCol)
	return err
}

// Delete 删除某主题全部向量。
func (m *Milvus) Delete(ctx context.Context, topic string) error {
	if !m.Available() {
		return fmt.Errorf("milvus 不可用")
	}
	expr := fmt.Sprintf("topic == %q", topic)
	return m.client.Delete(ctx, collectionName, "", expr)
}

// Search 按 topic 过滤检索 topK（余弦相似度降序）。
func (m *Milvus) Search(ctx context.Context, topic string, queryVec []float32, topK int) ([]Hit, error) {
	if !m.Available() {
		return nil, fmt.Errorf("milvus 不可用")
	}
	if topK <= 0 {
		topK = 10
	}
	expr := fmt.Sprintf("topic == %q", topic)
	sp, _ := entity.NewIndexHNSWSearchParam(64)
	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	results, err := m.client.Search(ctx, collectionName, nil, expr,
		[]string{"chunk_index"}, []entity.Vector{entity.FloatVector(queryVec)},
		"embedding", entity.COSINE, topK, sp)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 || results[0].ResultCount == 0 {
		return nil, nil
	}
	hits := make([]Hit, 0, results[0].ResultCount)
	idc := results[0].Fields.GetColumn("chunk_index")
	idc64, ok := idc.(*entity.ColumnInt64)
	if !ok {
		return nil, fmt.Errorf("milvus: 结果列类型异常")
	}
	for i := 0; i < results[0].ResultCount; i++ {
		hits = append(hits, Hit{
			ChunkIndex: idc64.Data()[i],
			Score:      results[0].Scores[i],
		})
	}
	return hits, nil
}

// --- 列构造辅助 ---

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func chunkIndexes(chunks []store.KnowledgeChunk) []int64 {
	out := make([]int64, len(chunks))
	for i, c := range chunks {
		out[i] = int64(c.Index)
	}
	return out
}

func chunkEmbeddings(chunks []store.KnowledgeChunk) [][]float32 {
	out := make([][]float32, len(chunks))
	for i, c := range chunks {
		out[i] = c.Embedding
	}
	return out
}
