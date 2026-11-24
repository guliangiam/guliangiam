// Package rebac 是 ReBAC 关系元组求值器实现。
//
// ADR-024 定义：存储复用 PostgreSQL（PostgreSQL/MySQL 双基线一致 schema），
// 求值器在内存 BFS 展开 —— 一次性 FetchAllTuples 后用邻接表加速查找。
// 核心数据结构：
//
//      tupleStore  →  求值器内部构建的索引，keyed by (subject, relation)
//                    每个 entry 列出该 subject 下某 relation 能到达的 object 集合
//
//      BFS frontier → 每一层把当前 node 的 next-hop tuple 拉出来，depth+1 进下一层
//      visited      → 防止环（user A member group B, group B member user A 这种场景）
//
// Zookie 机制：每次写入 bump namespace.revision_counter 作为 zookie；
// 读请求携带 atZookie 保证快照边界；响应返回本次快照的 zookie。

package rebac

import (
	"context"
	"fmt"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
)

// key 是求值器内部用来唯一标识 BFS 状态的三元组。
// subjectRelation 可选（表达 membership 链的中间节点），subject 非空。
type key struct {
	subject string
	rel     string
	obj     string
	hasSR   bool
	subjRel string
}

func (k key) String() string {
	if k.hasSR {
		return fmt.Sprintf("(%s [%s] %s %s)", k.subject, k.subjRel, k.rel, k.obj)
	}
	return fmt.Sprintf("(%s %s %s)", k.subject, k.rel, k.obj)
}

// store 是求值器的内存索引。
type store struct {
	bySubjectRelation map[string]map[string][]string // subject → relation → objects
	byObjectRelation  map[string]map[string][]string // object  → relation → subjects
	tupleIndex        map[key]bool
}

func newStore() *store {
	return &store{
		bySubjectRelation: map[string]map[string][]string{},
		byObjectRelation:  map[string]map[string][]string{},
		tupleIndex:        map[key]bool{},
	}
}

func (s *store) add(t *domain.Tuple) {
	if s.bySubjectRelation[t.Subject] == nil {
		s.bySubjectRelation[t.Subject] = map[string][]string{}
	}
	s.bySubjectRelation[t.Subject][t.Relation] = append(s.bySubjectRelation[t.Subject][t.Relation], t.Object)

	if s.byObjectRelation[t.Object] == nil {
		s.byObjectRelation[t.Object] = map[string][]string{}
	}
	s.byObjectRelation[t.Object][t.Relation] = append(s.byObjectRelation[t.Object][t.Relation], t.Subject)

	sr := ""
	hasSR := t.SubjectRelation != nil
	if hasSR {
		sr = *t.SubjectRelation
	}
	s.tupleIndex[key{t.Subject, t.Relation, t.Object, hasSR, sr}] = true
}

// Engine 是 ADR-024 的核心 ReBAC 求值器。纯内存、无锁、无持久化状态。
type Engine struct {
	store    domain.TupleStore
	maxDepth int
}

// NewEngine 构造一个 Evaluator，store 是 Repository（已满足 TupleStore 接口）。
func NewEngine(store domain.TupleStore) *Engine {
	return &Engine{store: store, maxDepth: domain.DefaultMaxDepth}
}

func (e *Engine) withDepth(d int) int {
	if d <= 0 {
		return domain.DefaultMaxDepth
	}
	if d > 32 {
		return 32 // 硬上限防止误配炸内存
	}
	return d
}

// resolveZookie 决定本次读请求用哪个快照边界：
//   - atZookie 非空 → 用它（用户指定了写后读一致的快照）
//   - atZookie 为空 → 拉完数据后获取 namespace 当前最新 zookie
func (e *Engine) resolveZookie(ctx context.Context, orgID, nsID, atZookie string) (string, error) {
	if atZookie != "" {
		return atZookie, nil
	}
	return e.store.GetLatestZookie(ctx, orgID, nsID)
}

func (e *Engine) buildStore(ctx context.Context, orgID, nsID, atZookie string) (*store, error) {
	tuples, err := e.store.FetchAllTuples(ctx, orgID, nsID, atZookie)
	if err != nil {
		return nil, err
	}
	s := newStore()
	for _, t := range tuples {
		s.add(t)
	}
	return s, nil
}

// Check 求值 user 是否对 object 有 relation 权限。
// 算法：BFS + 环检测 + 深度受控 + Zookie 快照边界。
//
// 关键 insight：中间 hop 的 relation 可以不同于用户请求的 relation。
// 例：Check(user:u2, write, folder:*) 路径是
//
//	user:u2 → has_role → role:editor → write → folder:*
//	          中间 relation ↑            ↑ 最终匹配 relation
func (e *Engine) Check(ctx context.Context, req domain.CheckRequest) (*domain.CheckResult, error) {
	depthLimit := e.withDepth(req.DepthLimit)

	// 1. 先拿最新 zookie（可能是用户指定的，也可能是 namespace 当前最新）
	zookie, err := e.resolveZookie(ctx, req.OrganizationID, req.NamespaceID, req.AtZookie)
	if err != nil {
		return nil, err
	}

	// 2. 拉取 atZookie ≤ zookie 的元组（构建快照）
	s, err := e.buildStore(ctx, req.OrganizationID, req.NamespaceID, zookie)
	if err != nil {
		return nil, err
	}

	// 3. 直接命中短路
	if rels, ok := s.bySubjectRelation[req.Tuple.Subject]; ok {
		if objs, ok2 := rels[req.Tuple.Relation]; ok2 {
			for _, obj := range objs {
				if obj == req.Tuple.Object {
					return &domain.CheckResult{
						Allowed: true,
						Trace: []string{
							fmt.Sprintf("(%s %s %s)", req.Tuple.Subject, req.Tuple.Relation, req.Tuple.Object),
							"✓ direct match",
						},
						Zookie: zookie,
					}, nil
				}
			}
		}
	}

	targetRel := req.Tuple.Relation
	targetObj := req.Tuple.Object

	trace := []string{fmt.Sprintf("target: (* %s %s)", targetRel, targetObj)}
	visited := map[string]bool{}

	queue := []string{req.Tuple.Subject}
	visited[req.Tuple.Subject] = true
	trace = append(trace, fmt.Sprintf("level 0: [%s]", req.Tuple.Subject))

	depth := 0
	for len(queue) > 0 && depth < depthLimit {
		depth++
		nextQueue := []string{}

		for _, subject := range queue {
			if rels, ok := s.bySubjectRelation[subject]; ok {
				if objs, ok2 := rels[targetRel]; ok2 {
					for _, obj := range objs {
						if obj == targetObj {
							trace = append(trace,
								fmt.Sprintf("level %d match: %s → %s %s %s",
									depth, subject, targetRel, targetObj, "✓"))
							return &domain.CheckResult{Allowed: true, Trace: trace, Zookie: zookie}, nil
						}
					}
				}
			}

			if allRels, ok := s.bySubjectRelation[subject]; ok {
				for _, objs := range allRels {
					for _, obj := range objs {
						if !visited[obj] {
							visited[obj] = true
							nextQueue = append(nextQueue, obj)
						}
					}
				}
			}
		}

		if len(nextQueue) == 0 {
			break
		}
		trace = append(trace, fmt.Sprintf("level %d: %d new subjects", depth, len(nextQueue)))
		queue = nextQueue
	}

	trace = append(trace, fmt.Sprintf("✗ no path within depth %d", depthLimit))
	return &domain.CheckResult{Allowed: false, Trace: trace, Zookie: zookie}, nil
}

// Expand 展开所有对 (relation, object) 有此 relation 的 subject。
func (e *Engine) Expand(ctx context.Context, req domain.ExpandRequest) (*domain.ExpandResult, error) {
	depthLimit := e.withDepth(req.DepthLimit)

	zookie, err := e.resolveZookie(ctx, req.OrganizationID, req.NamespaceID, req.AtZookie)
	if err != nil {
		return nil, err
	}

	s, err := e.buildStore(ctx, req.OrganizationID, req.NamespaceID, zookie)
	if err != nil {
		return nil, err
	}

	type node struct {
		subject string
		depth   int
	}

	visited := map[string]bool{}
	root := domain.ExpandNode{Subject: req.Object, Expanded: true}
	frontier := []node{{subject: req.Object, depth: 0}}

	var build func(string, int) []domain.ExpandNode
	build = func(subj string, depth int) []domain.ExpandNode {
		if depth > depthLimit {
			return nil
		}
		rels, ok := s.byObjectRelation[subj]
		if !ok {
			return nil
		}
		subjects := rels[req.Relation]
		var children []domain.ExpandNode
		for _, nextSubj := range subjects {
			if visited[nextSubj] {
				continue
			}
			visited[nextSubj] = true
			child := domain.ExpandNode{Subject: nextSubj}
			descendants := build(nextSubj, depth+1)
			if len(descendants) > 0 {
				child.Children = descendants
				child.Expanded = true
			}
			children = append(children, child)
		}
		return children
	}

	root.Children = build(req.Object, 0)
	_ = frontier
	return &domain.ExpandResult{Root: root, Zookie: zookie}, nil
}

// BatchCheck 复用同一份 store（同一次 zookie 快照）批量求值多条 Check。
func (e *Engine) BatchCheck(ctx context.Context, orgID, nsID string, items []domain.TupleKey, atZookie string) ([]domain.CheckResult, error) {
	// 取一次快照，所有 item 共享同一份 store
	zookie, err := e.resolveZookie(ctx, orgID, nsID, atZookie)
	if err != nil {
		return nil, err
	}
	s, err := e.buildStore(ctx, orgID, nsID, zookie)
	if err != nil {
		return nil, err
	}

	depthLimit := e.withDepth(0) // 默认深度

	results := make([]domain.CheckResult, len(items))
	for i, t := range items {
		results[i] = e.checkWithStore(s, t, depthLimit, zookie)
	}
	return results, nil
}

// checkWithStore 是 BatchCheck 复用 store 后的轻量 Check 实现（不走 resolveZookie + buildStore）。
func (e *Engine) checkWithStore(s *store, t domain.TupleKey, depthLimit int, zookie string) domain.CheckResult {
	// 直接命中短路
	if rels, ok := s.bySubjectRelation[t.Subject]; ok {
		if objs, ok2 := rels[t.Relation]; ok2 {
			for _, obj := range objs {
				if obj == t.Object {
					return domain.CheckResult{Allowed: true, Zookie: zookie,
						Trace: []string{fmt.Sprintf("(%s %s %s) ✓ direct", t.Subject, t.Relation, t.Object)}}
				}
			}
		}
	}

	targetRel := t.Relation
	targetObj := t.Object
	trace := []string{fmt.Sprintf("target: (* %s %s)", targetRel, targetObj)}
	visited := map[string]bool{}

	queue := []string{t.Subject}
	visited[t.Subject] = true
	trace = append(trace, fmt.Sprintf("level 0: [%s]", t.Subject))

	depth := 0
	for len(queue) > 0 && depth < depthLimit {
		depth++
		nextQueue := []string{}
		for _, subject := range queue {
			if rels, ok := s.bySubjectRelation[subject]; ok {
				if objs, ok2 := rels[targetRel]; ok2 {
					for _, obj := range objs {
						if obj == targetObj {
							trace = append(trace, fmt.Sprintf("level %d match: %s %s %s ✓", depth, subject, targetRel, targetObj))
							return domain.CheckResult{Allowed: true, Trace: trace, Zookie: zookie}
						}
					}
				}
			}
			if allRels, ok := s.bySubjectRelation[subject]; ok {
				for _, objs := range allRels {
					for _, obj := range objs {
						if !visited[obj] {
							visited[obj] = true
							nextQueue = append(nextQueue, obj)
						}
					}
				}
			}
		}
		if len(nextQueue) == 0 {
			break
		}
		trace = append(trace, fmt.Sprintf("level %d: %d new", depth, len(nextQueue)))
		queue = nextQueue
	}

	trace = append(trace, fmt.Sprintf("✗ no path within %d", depthLimit))
	return domain.CheckResult{Allowed: false, Trace: trace, Zookie: zookie}
}

var _ domain.Evaluator = (*Engine)(nil)
