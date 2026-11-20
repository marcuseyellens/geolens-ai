// Package omnigraph 实现基线 WBS WP-2「市场图谱底座」的本地可运行内核：
// typed ontology（2.2）、分支-合入管线（2.3）、时间旅行查询（2.4）、混合检索（2.5）。
//
// 说明：WBS 允许「基础设施评估不通过则退回 DuckDB 方案」（WP-2.1）。本仓库
// 以纯 Go 实现等价的五维模型（市场 × 品牌 × 问题 × 引用 × 时间），不引入外部
// 图数据库与 AGPL 代码；Omnigraph 可作为 GraphStore 的一个实现替换进来。
package omnigraph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/answerlens/answerlens/domain"
)

var (
	ErrSchemaViolation = errors.New("schema violation: 节点或边不符合本体定义")
	ErrMainReadOnly    = errors.New("collector identity cannot write main: 采集身份禁止直接写 main 分支")
	ErrUnknownBranch   = errors.New("unknown branch")
	ErrNothingToMerge  = errors.New("nothing to merge")
)

// NodeKind 是本体允许的 7 类节点。
type NodeKind string

const (
	NodeMarket   NodeKind = "Market"
	NodeBrand    NodeKind = "Brand"
	NodeQuestion NodeKind = "Question"
	NodeRun      NodeKind = "Run"
	NodeMention  NodeKind = "Mention" // 独立成节点（WP-2.2 验收）
	NodeCitation NodeKind = "Citation"
	NodeSource   NodeKind = "Source"
)

// AllNodeKinds 是本体注册表的固定顺序。
func AllNodeKinds() []NodeKind {
	return []NodeKind{NodeMarket, NodeBrand, NodeQuestion, NodeRun, NodeMention, NodeCitation, NodeSource}
}

// EdgeKind 是本体允许的边。
type EdgeKind string

const (
	EdgeInMarket     EdgeKind = "IN_MARKET"
	EdgeAsks         EdgeKind = "ASKS"
	EdgeProduces     EdgeKind = "PRODUCES"
	EdgeHasMention   EdgeKind = "HAS_MENTION"
	EdgeMentions     EdgeKind = "MENTIONS"
	EdgeCites        EdgeKind = "CITES"
	EdgeFromSource   EdgeKind = "FROM_SOURCE"
	EdgeCompetitorOf EdgeKind = "COMPETITOR_OF"
)

// allowedEdges 是 schema：from kind -> to kind -> 允许的边。
var allowedEdges = map[NodeKind]map[NodeKind]EdgeKind{
	NodeBrand:    {NodeMarket: EdgeInMarket, NodeBrand: EdgeCompetitorOf},
	NodeQuestion: {NodeMarket: EdgeInMarket},
	NodeRun:      {NodeQuestion: EdgeAsks},
	NodeMention:  {NodeRun: EdgeHasMention, NodeBrand: EdgeMentions},
	NodeCitation: {NodeRun: EdgeProduces, NodeSource: EdgeFromSource},
}

// Node 是带类型的图节点。Props 保留各节点的业务字段。
type Node struct {
	ID        string            `json:"id"`
	Kind      NodeKind          `json:"kind"`
	Label     string            `json:"label"`
	Props     map[string]string `json:"props,omitempty"`
	ValidFrom time.Time         `json:"valid_from"`
	ValidTo   *time.Time        `json:"valid_to,omitempty"`
}

// Edge 是带类型的有向边，同样带有效时间以支持时间旅行。
type Edge struct {
	From      string     `json:"from"`
	To        string     `json:"to"`
	Kind      EdgeKind   `json:"kind"`
	ValidFrom time.Time  `json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to,omitempty"`
}

// MentionNodeProps 把三态布尔写到独立 Mention 节点上（PRD F-203）。
func MentionNodeProps(m domain.Mention, runID string) map[string]string {
	return map[string]string{
		"run_id":      runID,
		"brand_id":    m.BrandID,
		"mentioned":   boolStr(m.Mentioned),
		"recommended": boolStr(m.Recommended),
		"linked":      boolStr(m.Linked),
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Graph 是某个分支/时点下的图快照。
type Graph struct {
	Nodes map[string]Node `json:"nodes"`
	Edges []Edge          `json:"edges"`
}

func newGraph() *Graph {
	return &Graph{Nodes: map[string]Node{}, Edges: []Edge{}}
}

// ValidateNode 是 schema 校验：不合即拒绝写入（WP-2.2 验收）。
func ValidateNode(n Node) error {
	valid := false
	for _, k := range AllNodeKinds() {
		if n.Kind == k {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("%w: 未知节点类型 %q", ErrSchemaViolation, n.Kind)
	}
	if strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("%w: 节点 ID 为空", ErrSchemaViolation)
	}
	if n.ValidFrom.IsZero() {
		return fmt.Errorf("%w: 节点 %s 缺少 valid_from（时间旅行需要）", ErrSchemaViolation, n.ID)
	}
	// Mention 节点强制要求三态布尔，避免退化为内嵌字段。
	if n.Kind == NodeMention {
		for _, k := range []string{"mentioned", "recommended", "linked"} {
			if _, ok := n.Props[k]; !ok {
				return fmt.Errorf("%w: Mention 节点缺少三态字段 %s", ErrSchemaViolation, k)
			}
		}
	}
	return nil
}

// ValidateEdge 校验边两端存在且类型组合在注册表内。
func ValidateEdge(g *Graph, e Edge) error {
	from, ok := g.Nodes[e.From]
	if !ok {
		return fmt.Errorf("%w: 起点 %s 不存在", ErrSchemaViolation, e.From)
	}
	to, ok := g.Nodes[e.To]
	if !ok {
		return fmt.Errorf("%w: 终点 %s 不存在", ErrSchemaViolation, e.To)
	}
	expected, ok := allowedEdges[from.Kind][to.Kind]
	if !ok {
		return fmt.Errorf("%w: 不允许 %s -> %s", ErrSchemaViolation, from.Kind, to.Kind)
	}
	if expected != e.Kind {
		return fmt.Errorf("%w: %s -> %s 应为 %s 而非 %s", ErrSchemaViolation, from.Kind, to.Kind, expected, e.Kind)
	}
	return nil
}

// Identity 是分支写入身份。Collector 禁止写 main（WP-2.3 验收）。
type Identity string

const (
	IdentityCollector Identity = "collector"
	IdentityReviewer  Identity = "reviewer"
	IdentityAdmin     Identity = "admin"
)

// Branch 是一个可整体回滚的采集分支。
type Branch struct {
	ID        string    `json:"id"`
	Graph     *Graph    `json:"graph"`
	CreatedBy Identity  `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Merged    bool      `json:"merged"`
}

// Store 是市场图谱的最小运行时：main + 若干 batch 分支 + 合入日志。
type Store struct {
	mu       sync.RWMutex
	clock    func() time.Time
	main     *Graph
	branches map[string]*Branch
	history  []MergeRecord
}

// MergeRecord 让合入可整体回滚（WP-2.3 验收）。
type MergeRecord struct {
	ID         string    `json:"id"`
	BranchID   string    `json:"branch_id"`
	Operator   Identity  `json:"operator"`
	At         time.Time `json:"at"`
	NodeIDs    []string  `json:"node_ids"`
	EdgeCount  int       `json:"edge_count"`
	RolledBack bool      `json:"rolled_back"`
}

func NewStore(clock func() time.Time) *Store {
	if clock == nil {
		clock = time.Now
	}
	return &Store{
		clock: clock, main: newGraph(),
		branches: map[string]*Branch{}, history: []MergeRecord{},
	}
}

// CreateBranch 创建采集批次分支。
func (s *Store) CreateBranch(id string, by Identity) (*Branch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.branches[id]; exists {
		return nil, fmt.Errorf("branch %s already exists", id)
	}
	b := &Branch{ID: id, Graph: newGraph(), CreatedBy: by, CreatedAt: s.clock()}
	s.branches[id] = b
	return b, nil
}

// WriteNode 按身份与分支策略写入节点；schema 不合即拒绝。
func (s *Store) WriteNode(branchID string, by Identity, n Node) error {
	if err := ValidateNode(n); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.writableGraph(branchID, by)
	if err != nil {
		return err
	}
	g.Nodes[n.ID] = n
	return nil
}

// WriteEdge 写入边，校验两端类型。
func (s *Store) WriteEdge(branchID string, by Identity, e Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.writableGraph(branchID, by)
	if err != nil {
		return err
	}
	if e.ValidFrom.IsZero() {
		e.ValidFrom = s.clock()
	}
	if err := ValidateEdge(g, e); err != nil {
		return err
	}
	g.Edges = append(g.Edges, e)
	return nil
}

// writableGraph 是权限策略唯一入口：collector 写 main 一律被引擎拒绝。
func (s *Store) writableGraph(branchID string, by Identity) (*Graph, error) {
	if branchID == "main" {
		if by != IdentityAdmin && by != IdentityReviewer {
			return nil, fmt.Errorf("%w (identity=%s)", ErrMainReadOnly, by)
		}
		return s.main, nil
	}
	b, ok := s.branches[branchID]
	if !ok {
		return nil, ErrUnknownBranch
	}
	if b.Merged {
		return nil, fmt.Errorf("branch %s 已合入，不可再写", branchID)
	}
	return b.Graph, nil
}

// Merge 人工审核后把分支合入 main；只有 reviewer/admin 可执行。
func (s *Store) Merge(branchID string, by Identity) (*MergeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if by != IdentityAdmin && by != IdentityReviewer {
		return nil, fmt.Errorf("%w: 合入需要审核身份", ErrMainReadOnly)
	}
	b, ok := s.branches[branchID]
	if !ok {
		return nil, ErrUnknownBranch
	}
	if b.Merged {
		return nil, ErrNothingToMerge
	}
	rec := MergeRecord{
		ID: fmt.Sprintf("merge-%d", len(s.history)+1), BranchID: branchID,
		Operator: by, At: s.clock(), EdgeCount: len(b.Graph.Edges),
	}
	for id, n := range b.Graph.Nodes {
		s.main.Nodes[id] = n
		rec.NodeIDs = append(rec.NodeIDs, id)
	}
	s.main.Edges = append(s.main.Edges, b.Graph.Edges...)
	sort.Strings(rec.NodeIDs)
	b.Merged = true
	s.history = append(s.history, rec)
	return &rec, nil
}

// RollbackMerge 按记录整体回滚一次合入。
func (s *Store) RollbackMerge(mergeID string, by Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if by != IdentityAdmin {
		return errors.New("只有 admin 可回滚合入")
	}
	for i := range s.history {
		rec := &s.history[i]
		if rec.ID != mergeID || rec.RolledBack {
			continue
		}
		for _, id := range rec.NodeIDs {
			delete(s.main.Nodes, id)
		}
		// 去掉本次合入带进来的边（按尾部计数回退）
		if rec.EdgeCount <= len(s.main.Edges) {
			s.main.Edges = s.main.Edges[:len(s.main.Edges)-rec.EdgeCount]
		}
		rec.RolledBack = true
		return nil
	}
	return fmt.Errorf("merge %s 不存在或已回滚", mergeID)
}

// MergeHistory 返回合入日志。
func (s *Store) MergeHistory() []MergeRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MergeRecord, len(s.history))
	copy(out, s.history)
	return out
}

// ---- WP-2.4 时间旅行 ----

// asOf 判断实体在时点 t 是否有效（半开区间 [from, to)）。
func asOf(from time.Time, to *time.Time, t time.Time) bool {
	if t.Before(from) {
		return false
	}
	return to == nil || t.Before(*to)
}

// At 返回 main 在历史时点 t 的图快照；任意历史日期可重算市场基线。
func (s *Store) At(t time.Time) *Graph {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := newGraph()
	for id, n := range s.main.Nodes {
		if asOf(n.ValidFrom, n.ValidTo, t) {
			out.Nodes[id] = n
		}
	}
	for _, e := range s.main.Edges {
		if asOf(e.ValidFrom, e.ValidTo, t) {
			if _, ok := out.Nodes[e.From]; ok {
				if _, ok := out.Nodes[e.To]; ok {
					out.Edges = append(out.Edges, e)
				}
			}
		}
	}
	return out
}

// CloseNode 给节点打上失效时间，用于「时点重算与当日快照数字一致可解释」。
func (s *Store) CloseNode(id string, at time.Time, by Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if by != IdentityAdmin && by != IdentityReviewer {
		return fmt.Errorf("%w (identity=%s)", ErrMainReadOnly, by)
	}
	n, ok := s.main.Nodes[id]
	if !ok {
		return ErrUnknownBranch
	}
	n.ValidTo = &at
	s.main.Nodes[id] = n
	return nil
}

// ---- WP-2.5 混合检索 ----

// HybridQuery 一次查询组合结构约束 + 全文 + 相似度。
type HybridQuery struct {
	Kinds     []NodeKind `json:"kinds"`
	Text      string     `json:"text"`       // 全文关键词
	SimilarTo string     `json:"similar_to"` // 相似度锚点（按 label/prop 词面重合）
	MarketID  string     `json:"market_id"`  // 结构约束：限定某市场内的品牌
	Limit     int        `json:"limit"`
}

// HybridHit 是混合检索的一行，带可解释的分项得分。
type HybridHit struct {
	ID          string   `json:"id"`
	Kind        NodeKind `json:"kind"`
	Label       string   `json:"label"`
	StructScore float64  `json:"struct_score"`
	TextScore   float64  `json:"text_score"`
	SimScore    float64  `json:"sim_score"`
	Score       float64  `json:"score"`
}

// HybridSearch 结构 + 全文 + 相似度一次查询（WP-2.5 验收：单请求返回）。
func (s *Store) HybridSearch(q HybridQuery) []HybridHit {
	g := s.At(s.clock())
	structural := map[string]bool{}
	if q.MarketID != "" {
		// 与市场直接相连的品牌节点满足结构约束
		for _, e := range g.Edges {
			if e.Kind == EdgeInMarket && e.To == q.MarketID {
				structural[e.From] = true
			}
		}
	}
	anchor := ""
	if q.SimilarTo != "" {
		if n, ok := g.Nodes[q.SimilarTo]; ok {
			anchor = strings.ToLower(n.Label + " " + flattenProps(n.Props))
		}
	}
	kindSet := map[NodeKind]bool{}
	for _, k := range q.Kinds {
		kindSet[k] = true
	}
	needle := strings.ToLower(strings.TrimSpace(q.Text))
	hits := []HybridHit{}
	for id, n := range g.Nodes {
		if len(kindSet) > 0 && !kindSet[n.Kind] {
			continue
		}
		blob := strings.ToLower(n.Label + " " + flattenProps(n.Props))
		h := HybridHit{ID: id, Kind: n.Kind, Label: n.Label}
		if q.MarketID == "" || structural[id] || n.Kind == NodeMarket {
			h.StructScore = 1
		}
		if needle != "" && strings.Contains(blob, needle) {
			h.TextScore = 1
		}
		if anchor != "" {
			h.SimScore = jaccard(anchor, blob)
		}
		// 三类信号任一命中即入选；同时命中时得分累加，体现「混合」。
		if h.StructScore+h.TextScore+h.SimScore == 0 {
			continue
		}
		h.Score = h.StructScore + h.TextScore + h.SimScore
		hits = append(hits, h)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
	if q.Limit > 0 && len(hits) > q.Limit {
		hits = hits[:q.Limit]
	}
	return hits
}

func flattenProps(p map[string]string) string {
	parts := make([]string, 0, len(p))
	for _, v := range p {
		parts = append(parts, v)
	}
	return strings.Join(parts, " ")
}

func jaccard(a, b string) float64 {
	sa, sb := tokenSet(a), tokenSet(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for k := range sa {
		if sb[k] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '/' || r == ':' || r == '-' || r == '_'
	}) {
		if len(f) > 1 {
			out[f] = true
		}
	}
	return out
}

// Snapshot 返回当前 main 的节点/边数量，供 API 与时间旅行对账。
func (s *Store) Snapshot() (int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.main.Nodes), len(s.main.Edges)
}
