package omnigraph

import (
	"errors"
	"testing"
	"time"

	"github.com/geolens/geolens/domain"
)

func clockAt(t time.Time) func() time.Time { return func() time.Time { return t } }

func seedGraph(t *testing.T, s *Store, branch string, at time.Time) {
	t.Helper()
	nodes := []Node{
		{ID: "mkt:aeo", Kind: NodeMarket, Label: "AEO 平台市场", ValidFrom: at},
		{ID: "brand:geolens", Kind: NodeBrand, Label: "GeoLens AEO 平台", ValidFrom: at,
			Props: map[string]string{"category": "AEO 平台"}},
		{ID: "brand:rival", Kind: NodeBrand, Label: "竞品 AEO 工具", ValidFrom: at},
		{ID: "q:best", Kind: NodeQuestion, Label: "最好的 AEO 平台", ValidFrom: at},
		{ID: "run:1", Kind: NodeRun, Label: "2026-W45 采集批次", ValidFrom: at},
		{ID: "mention:1", Kind: NodeMention, Label: "GeoLens 提及", ValidFrom: at,
			Props: MentionNodeProps(domain.Mention{
				BrandID: "brand:geolens", Mentioned: true, Recommended: true, Linked: false,
			}, "run:1")},
		{ID: "src:industry", Kind: NodeSource, Label: "行业独立评测站", ValidFrom: at},
		{ID: "cite:1", Kind: NodeCitation, Label: "https://example.com/review", ValidFrom: at},
	}
	for _, n := range nodes {
		if err := s.WriteNode(branch, IdentityCollector, n); err != nil {
			t.Fatalf("写入节点 %s 失败: %v", n.ID, err)
		}
	}
	edges := []Edge{
		{From: "brand:geolens", To: "mkt:aeo", Kind: EdgeInMarket},
		{From: "brand:rival", To: "mkt:aeo", Kind: EdgeInMarket},
		{From: "brand:geolens", To: "brand:rival", Kind: EdgeCompetitorOf},
		{From: "q:best", To: "mkt:aeo", Kind: EdgeInMarket},
		{From: "run:1", To: "q:best", Kind: EdgeAsks},
		{From: "mention:1", To: "run:1", Kind: EdgeHasMention},
		{From: "mention:1", To: "brand:geolens", Kind: EdgeMentions},
		{From: "cite:1", To: "run:1", Kind: EdgeProduces},
		{From: "cite:1", To: "src:industry", Kind: EdgeFromSource},
	}
	for _, e := range edges {
		if err := s.WriteEdge(branch, IdentityCollector, e); err != nil {
			t.Fatalf("写入边 %s->%s 失败: %v", e.From, e.To, err)
		}
	}
}

// WP-2.2：schema 不合即拒绝写入；Mention 独立节点必须带三态布尔。
func TestOntologySchemaRejectsInvalidWrites(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	s := NewStore(clockAt(now))
	if _, err := s.CreateBranch("batch/1", IdentityCollector); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteNode("batch/1", IdentityCollector, Node{
		ID: "x", Kind: "UnknownKind", ValidFrom: now,
	}); !errors.Is(err, ErrSchemaViolation) {
		t.Error("未知节点类型未被拒绝")
	}
	if err := s.WriteNode("batch/1", IdentityCollector, Node{
		ID: "mention:x", Kind: NodeMention, ValidFrom: now,
		Props: map[string]string{"mentioned": "true"},
	}); !errors.Is(err, ErrSchemaViolation) {
		t.Error("缺少三态字段的 Mention 节点未被拒绝")
	}
	seedGraph(t, s, "batch/1", now)
	n, _ := s.branches["batch/1"].Graph.Nodes["mention:1"], 0
	_ = n
	m := s.branches["batch/1"].Graph.Nodes["mention:1"]
	if m.Kind != NodeMention || m.Props["mentioned"] != "true" || m.Props["recommended"] != "true" || m.Props["linked"] != "false" {
		t.Fatalf("Mention 三态未独立存储: %+v", m)
	}
	// 非法边组合（品牌指向来源）被拒绝
	if err := s.WriteEdge("batch/1", IdentityCollector, Edge{
		From: "brand:geolens", To: "src:industry", Kind: EdgeFromSource,
	}); !errors.Is(err, ErrSchemaViolation) {
		t.Error("本体未注册的边类型未被拒绝")
	}
}

// WP-2.3：collector 身份写 main 被引擎拒绝（权限策略）。
func TestCollectorCannotWriteMain(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	s := NewStore(clockAt(now))
	err := s.WriteNode("main", IdentityCollector, Node{
		ID: "brand:direct", Kind: NodeBrand, Label: "直写", ValidFrom: now,
	})
	if !errors.Is(err, ErrMainReadOnly) {
		t.Fatalf("collector 直写 main 未被引擎拒绝: %v", err)
	}
	// 采集身份只能写 batch 分支，再由 reviewer 合入
	if _, err := s.CreateBranch("batch/1", IdentityCollector); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteNode("batch/1", IdentityCollector, Node{
		ID: "brand:ok", Kind: NodeBrand, Label: "合规写入", ValidFrom: now,
	}); err != nil {
		t.Fatalf("分支写入失败: %v", err)
	}
	if _, err := s.Merge("batch/1", IdentityCollector); !errors.Is(err, ErrMainReadOnly) {
		t.Error("collector 不应有权合入 main")
	}
	rec, err := s.Merge("batch/1", IdentityReviewer)
	if err != nil {
		t.Fatalf("reviewer 合入失败: %v", err)
	}
	if nodes, _ := s.Snapshot(); nodes != 1 {
		t.Errorf("合入后 main 节点数应为 1，实际 %d", nodes)
	}
	// 合入可整体回滚
	if err := s.RollbackMerge(rec.ID, IdentityAdmin); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	if nodes, _ := s.Snapshot(); nodes != 0 {
		t.Errorf("回滚后 main 应为空，实际 %d 节点", nodes)
	}
	if !s.MergeHistory()[0].RolledBack {
		t.Error("合入日志未标记回滚")
	}
}

// WP-2.4：任意历史日期可重算市场基线，与当日快照数字一致可解释。
func TestTimeTravelAsOfQuery(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	s := NewStore(clockAt(t2))
	if _, err := s.CreateBranch("batch/base", IdentityCollector); err != nil {
		t.Fatal(err)
	}
	seedGraph(t, s, "batch/base", t0)
	if _, err := s.Merge("batch/base", IdentityReviewer); err != nil {
		t.Fatal(err)
	}
	before := s.At(t0.Add(24 * time.Hour))
	if len(before.Nodes) != 8 {
		t.Fatalf("历史时点应重算到 8 节点，实际 %d", len(before.Nodes))
	}
	// 11/1 起把竞品节点标记为失效
	if err := s.CloseNode("brand:rival", t1, IdentityReviewer); err != nil {
		t.Fatal(err)
	}
	after := s.At(t2)
	if len(after.Nodes) != 7 {
		t.Errorf("失效后时点应为 7 节点，实际 %d", len(after.Nodes))
	}
	if _, ok := after.Nodes["brand:rival"]; ok {
		t.Error("已失效节点仍出现在时点快照")
	}
	// 时间旅行不得改写当前 main
	if nodes, _ := s.Snapshot(); nodes != 8 {
		t.Errorf("当前快照应保持 8 节点，实际 %d", nodes)
	}
}

// WP-2.5：结构 + 全文 + 相似度一次查询返回；「按品类语义 + 竞品关系查市场」。
func TestHybridSearchSingleRequest(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	s := NewStore(clockAt(now))
	if _, err := s.CreateBranch("batch/1", IdentityCollector); err != nil {
		t.Fatal(err)
	}
	seedGraph(t, s, "batch/1", now)
	if _, err := s.Merge("batch/1", IdentityReviewer); err != nil {
		t.Fatal(err)
	}
	hits := s.HybridSearch(HybridQuery{
		Kinds: []NodeKind{NodeBrand}, Text: "AEO", SimilarTo: "brand:geolens",
		MarketID: "mkt:aeo", Limit: 10,
	})
	if len(hits) < 2 {
		t.Fatalf("混合检索应返回至少 2 个品牌，实际 %d: %+v", len(hits), hits)
	}
	for _, h := range hits {
		if h.StructScore == 0 || h.TextScore == 0 {
			t.Errorf("混合命中应同时带结构与全文信号: %+v", h)
		}
	}
	// 相似度锚点自身应排最前
	if hits[0].ID != "brand:geolens" {
		t.Errorf("相似度锚点应排最前，实际 %s", hits[0].ID)
	}
	// 不存在的结构约束：不得把无关品牌当作命中（市场节点自身可入选）。
	strict := s.HybridSearch(HybridQuery{MarketID: "mkt:missing"})
	for _, h := range strict {
		if h.Kind != NodeMarket {
			t.Errorf("空市场不应返回品牌，实际返回 %s (%s)", h.ID, h.Kind)
		}
	}
}
