#!/usr/bin/env python3
"""P14 全链路 Trace 可视化报告生成器。

从 OTel stdouttrace (JSON) 输出中提取 spans，
生成 DOT 依赖图 + ASCII 时间条 + Markdown 报告。

用法：
  # 1. 生成 trace data（项目里跑）
  go test -run TestTraceCapture -v -count=1 ./internal/platform/middleware/...

  # 2. 生成报告（本脚本）
  python3 scripts/trace-visualize.py /tmp/trace_spans.json \
      --out docs/trace-report.md \
      --dot docs/trace-graphviz.dot

依赖：无外部依赖（标准库 only）

输入：OTel stdouttrace JSON（go test -run TestTraceCapture 输出）
输出：DOT 依赖图、ASCII 时间条、Markdown 报告
--check 行为：无 --check；手动生成工具（非门禁）
消费方：独立手动脚本；不纳入 verify-docs.sh
"""
import argparse, json, re, sys, os
from collections import defaultdict

def parse_spans(path):
    """解析 stdouttrace JSON（独立 JSON objects，一个 span 一个 { ... }）"""
    text = open(path).read()
    spans = []
    decoder = json.JSONDecoder()
    pos = 0
    while pos < len(text):
        while pos < len(text) and text[pos] in ' \t\n\r':
            pos += 1
        if pos >= len(text):
            break
        # 兼容两种格式：独立 { ... } 和 [ { ... } ]
        if text[pos] not in '{[' :
            pos += 1
            continue
        try:
            obj, end = decoder.raw_decode(text, pos)
            if isinstance(obj, dict):
                if 'SpanContext' in obj and 'Name' in obj:
                    spans.append(obj)
            elif isinstance(obj, list):
                for item in obj:
                    if isinstance(item, dict) and 'SpanContext' in item and 'Name' in item:
                        spans.append(item)
            pos = end
        except json.JSONDecodeError:
            pos += 1
    return spans

def ns_duration(span):
    """计算 span duration (ns)"""
    from datetime import datetime
    start = span.get('StartTime', '')
    end = span.get('EndTime', '')
    if not start or not end:
        return 0
    try:
        dt1 = datetime.fromisoformat(start)
        dt2 = datetime.fromisoformat(end)
        return max(0, int((dt2 - dt1).total_seconds() * 1e9))
    except:
        return 0

def build_tree(spans):
    """按 TraceID 分组 + 按 Parent SpanID 链接"""
    by_trace = defaultdict(list)
    for s in spans:
        tid = s.get('SpanContext', {}).get('TraceID', 'unknown')
        by_trace[tid].append(s)

    trees = []
    for tid, tspans in by_trace.items():
        by_span = {s['SpanContext']['SpanID']: s for s in tspans}
        children = defaultdict(list)
        roots = []
        for s in tspans:
            pid = s.get('Parent', {}).get('SpanID', '')
            if pid in ('', '0000000000000000'):
                roots.append(s)
            else:
                children[pid].append(s)
        trees.append((tid, roots, children, by_span))
    return trees

def fmt_ns(ns):
    if ns < 1000: return f"{ns}ns"
    if ns < 1_000_000: return f"{ns/1000:.1f}µs"
    if ns < 1_000_000_000: return f"{ns/1_000_000:.1f}ms"
    return f"{ns/1_000_000_000:.1f}s"

def ascii_timeline(spans, width=60):
    """ASCII 时间条图"""
    if not spans:
        return "(no spans)"

    # 计算绝对时间偏移（最小 start → 0）
    starts = {}
    for s in spans:
        # 简单处理：用 span 自己的 StartTime 字符串对比
        starts[s['SpanContext']['SpanID']] = s.get('StartTime', '')
    # 找最早的 start string
    min_start = min(starts.values()) if starts else ''

    rows = []
    max_dur = max((ns_duration(s) for s in spans), default=1)

    # 按 duration 降序
    spans_sorted = sorted(spans, key=lambda s: -ns_duration(s))

    rows.append(f"{'Span':<30} {'Duration':<12} Timeline")
    rows.append('-' * (30 + 12 + width))

    for s in spans_sorted:
        name = s['Name'][:28].ljust(28)
        dur = ns_duration(s)
        dur_str = fmt_ns(dur).ljust(12)
        # 时间条
        fill = max(1, int(dur / max_dur * width))
        bar = '█' * fill + '░' * (width - fill)
        rows.append(f"{name} {dur_str} {bar}")

    return '\n'.join(rows)

def dot_graph(spans, out_path):
    """生成 Graphviz DOT 文件"""
    lines = ['digraph Trace {', '  rankdir=TB;', '  node [shape=box, style="rounded,filled", fontname="Arial", fontsize=11];']

    # 颜色映射（按 span 类别）
    color_map = {
        'auth.jwt': '"#E8F5E9"',          # 绿
        'authz.casbin': '"#E3F2FD"',       # 蓝
        'db.': '"#FFF3E0"',                # 橙
        'auth.oauth2': '"#FCE4EC"',        # 粉
        'http': '"#F3E5F5"',               # 紫
    }
    def color(name):
        for prefix, c in color_map.items():
            if name.startswith(prefix):
                return c
        return '"#FAFAFA"'

    nodes = set()
    edges = []
    for s in spans:
        sid = s['SpanContext']['SpanID'][:8]
        tid = s['SpanContext']['TraceID'][:8]
        label = f'{s["Name"]}\\n{fmt_ns(ns_duration(s))}'
        c = color(s['Name'])
        if sid not in nodes:
            lines.append(f'  "{sid}" [label="{label}", fillcolor={c}];')
            nodes.add(sid)
        pid = s.get('Parent', {}).get('SpanID', '')
        if pid and pid != '0000000000000000':
            pid_short = pid[:8]
            edges.append(f'  "{pid_short}" -> "{sid}";')

    # 没有 parent 的 span → 挂到 fake root
    if edges:
        lines.append('  "root" [label="Trace Root", shape=ellipse, fillcolor="#90CAF9"];')
        for s in spans:
            pid = s.get('Parent', {}).get('SpanID', '')
            if pid in ('', '0000000000000000'):
                sid = s['SpanContext']['SpanID'][:8]
                if sid in nodes:
                    edges.append(f'  "root" -> "{sid}";')

    lines.extend(edges)
    lines.append('}')
    with open(out_path, 'w') as f:
        f.write('\n'.join(lines))
    return out_path

def md_report(trees, dot_path, md_path):
    """生成 Markdown 报告"""
    lines = [
        '# GuliangIAM P14 — OTel Trace 全链路可视化报告',
        '',
        f'生成时间：{__import__("datetime").datetime.now().strftime("%Y-%m-%d %H:%M:%S")}',
        '',
        f'Trace 数量：{len(trees)}',
        '',
        '---',
        '',
    ]

    for i, (tid, roots, children, by_span) in enumerate(trees, 1):
        all_spans = list(by_span.values())
        total_dur = sum(ns_duration(s) for s in all_spans)

        lines.append(f'## Trace #{i} — `{tid[:16]}...`')
        lines.append('')
        lines.append(f'- **总 span 数**：{len(all_spans)}')
        lines.append(f'- **总 duration**：{fmt_ns(total_dur)}')
        lines.append('')

        # ASCII 时间条
        lines.append('### 时间分布（按 duration 降序）')
        lines.append('')
        lines.append('```')
        lines.append(ascii_timeline(all_spans))
        lines.append('```')
        lines.append('')

        # DOT 图说明
        if i == 1 and dot_path:
            lines.append(f'### 依赖图（Graphviz DOT）')
            lines.append('')
            lines.append(f'> DOT 文件已生成：`{dot_path}`')
            lines.append('> 渲染为 PNG/SVG：`dot -Tpng trace-graphviz.dot -o trace-graphviz.png`')
            lines.append('')

        # 关键发现
        lines.append('### 关键发现')
        lines.append('')
        # 按 category 聚合
        categories = defaultdict(list)
        for s in all_spans:
            name = s['Name']
            cat = name.split('.')[0] + '.' + name.split('.')[1] if '.' in name else name
            categories[cat].append(ns_duration(s))

        # 找最长 span
        longest = max(all_spans, key=lambda s: ns_duration(s), default=None)
        if longest:
            lines.append(f'- **最长 span**：`{longest["Name"]}` — {fmt_ns(ns_duration(longest))}')

        # 按 category 汇总
        lines.append('')
        lines.append('| Category | Span Count | Total Duration | % of Root |')
        lines.append('|----------|-----------|----------------|-----------|')
        root_dur = ns_duration(roots[0]) if roots else total_dur
        for cat, durs in sorted(categories.items(), key=lambda x: -sum(x[1])):
            total = sum(durs)
            pct = f"{total/max(root_dur,1)*100:.1f}%"
            lines.append(f'| `{cat}` | {len(durs)} | {fmt_ns(total)} | {pct} |')

        lines.append('')
        lines.append('---')
        lines.append('')

    with open(md_path, 'w') as f:
        f.write('\n'.join(lines))
    return md_path

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('input', help='OTel stdouttrace JSON 文件路径')
    ap.add_argument('--out', '-o', default='docs/trace-report.md', help='Markdown 报告输出路径')
    ap.add_argument('--dot', '-d', default='docs/trace-graphviz.dot', help='Graphviz DOT 输出路径')
    args = ap.parse_args()

    spans = parse_spans(args.input)
    if not spans:
        print(f"❌ 没找到 span。确认 OTel stdouttrace 输出到了 {args.input}")
        sys.exit(1)

    print(f"✅ 解析到 {len(spans)} 个 spans")
    trees = build_tree(spans)
    print(f"✅ 按 TraceID 分组为 {len(trees)} 条 trace")

    # 生成 DOT
    os.makedirs(os.path.dirname(args.dot) or '.', exist_ok=True)
    dot_graph(spans, args.dot)
    print(f"✅ DOT 依赖图: {args.dot}")

    # 生成 Markdown
    os.makedirs(os.path.dirname(args.out) or '.', exist_ok=True)
    md_report(trees, args.dot, args.out)
    print(f"✅ Markdown 报告: {args.out}")

if __name__ == '__main__':
    main()
