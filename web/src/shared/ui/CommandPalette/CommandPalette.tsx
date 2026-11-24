import { useEffect, useMemo, useRef, useState } from "react";
import { Input, Modal } from "antd";
import type { InputRef } from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { menuGroups } from "@/features/navigation/menu";

type CommandKind = "navigate" | "resource" | "create";

type CommandItem = {
  id: string;
  kind: CommandKind;
  label: string;
  /** Secondary descriptive line — group name or detail. */
  group: string;
  /** Route path for navigate; resource detail route for resource/create. */
  target: string;
  /** Plain-text tokens used for ranking. */
  tokens: string[];
};

/**
 * Build a static command index from the sidebar menu.
 *
 * Reads `menuGroups` (already in AppShell.tsx) so the palette never drifts
 * from what the operator sees in the sidebar. The create shortcuts are
 * derived from "List of X" → "Create new X" by convention — a route
 * `/users` becomes a create shortcut labelled "Create User" targeting
 * `/users/new`.
 */
function buildIndex(): CommandItem[] {
  const items: CommandItem[] = [];

  for (const group of menuGroups) {
    for (const child of group.children) {
      items.push({
        id: `nav:${child.key}`,
        kind: "navigate",
        label: child.label,
        group: group.label,
        target: child.key,
        tokens: [child.label.toLowerCase(), child.key.toLowerCase(), group.label.toLowerCase()],
      });

      // Derive a "Create X" shortcut from list routes
      if (child.key.startsWith("/") && !child.key.includes(":") && child.key !== "/") {
        const singular = humanize(child.key.split("/").pop() ?? "");
        items.push({
          id: `create:${child.key}`,
          kind: "create",
          label: `Create ${singular}`,
          group: group.label,
          target: `${child.key}/new`,
          tokens: [
            `create ${singular}`.toLowerCase(),
            child.key.toLowerCase(),
            singular.toLowerCase(),
          ],
        });
      }
    }
  }

  return items;
}

function humanize(slug: string): string {
  return slug.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

/**
 * Simple fuzzy match: the haystack's score is the sum of the edit-distance
 * penalties across the query tokens. Lower = better. Results with every
 * token as a substring of *something* in the tokens array get rank 0.
 */
function rank(items: CommandItem[], query: string): CommandItem[] {
  const q = query.trim().toLowerCase();
  if (!q) return items.slice(0, 40); // First 40 entries when empty

  const terms = q.split(/\s+/);
  const scored = items
    .map((it) => {
      const hay = it.tokens.join(" ").toLowerCase();
      let score = 0;
      let ok = true;
      for (const term of terms) {
        if (!term) continue;
        const idx = hay.indexOf(term);
        if (idx === -1) {
          ok = false;
          score += 1000;
        } else {
          score += idx;
        }
      }
      return { it, score, ok };
    })
    .filter((entry) => entry.ok);

  scored.sort((a, b) => a.score - b.score);
  return scored.slice(0, 40).map((entry) => entry.it);
}

type CommandPaletteProps = {
  open: boolean;
  onClose: () => void;
};

export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const inputRef = useRef<InputRef | null>(null);
  const items = useMemo(() => buildIndex(), []);
  const ranked = useMemo(() => rank(items, query), [items, query]);

  // Reset state every time the modal opens. A render-time reset (rather than an
  // effect) keeps the previous query from reaching a paint of the reopened palette.
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (open) {
      setQuery("");
      setCursor(0);
    }
  }

  // Focus input on open
  useEffect(() => {
    if (open) {
      const id = setTimeout(() => inputRef.current?.focus(), 0);
      return () => clearTimeout(id);
    }
  }, [open]);

  function select(item: CommandItem) {
    navigate(item.target);
    onClose();
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      closable={false}
      width={640}
      title={null}
      centered
      styles={{ body: { padding: 0 } }}
      className="command-palette"
    >
      <div className="command-palette-input">
        <SearchOutlined />
        <Input
          ref={inputRef}
          placeholder={t("commandPalette.placeholder", "Search or jump to…")}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setCursor(0);
          }}
          onPressEnter={() => {
            const picked = ranked[cursor];
            if (picked) select(picked);
          }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") {
              e.preventDefault();
              setCursor((c) => Math.min(c + 1, ranked.length - 1));
            }
            if (e.key === "ArrowUp") {
              e.preventDefault();
              setCursor((c) => Math.max(c - 1, 0));
            }
          }}
          variant="borderless"
          style={{ fontSize: 16, padding: "12px 8px" }}
        />
      </div>

      {/* 命令面板采用 WAI-ARIA combobox/listbox 组合：原生 `<datalist>`/`<select>`
          无法承载自由文本过滤、键盘游标与分组，故只能显式声明 role。 */}
      {/* oxlint-disable jsx-a11y/prefer-tag-over-role */}
      <div className="command-palette-list" role="listbox">
        {ranked.length === 0 && (
          <div style={{ padding: "24px 16px", textAlign: "center", color: "#999" }}>
            {t("commandPalette.noResults", "No results")}
          </div>
        )}
        {ranked.map((item, i) => (
          <button
            key={item.id}
            role="option"
            aria-selected={i === cursor}
            onMouseEnter={() => setCursor(i)}
            onClick={() => select(item)}
            className={`command-palette-item${i === cursor ? " is-active" : ""}`}
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "flex-start",
              padding: "10px 16px",
              width: "100%",
              border: "none",
              background: i === cursor ? "var(--gl-color-bg-elevated, #f3f3f5)" : "transparent",
              cursor: "pointer",
              textAlign: "left",
            }}
          >
            <span style={{ fontWeight: 500 }}>{item.label}</span>
            <span style={{ color: "#999", fontSize: 12 }}>{item.group}</span>
          </button>
        ))}
      </div>
      {/* oxlint-enable jsx-a11y/prefer-tag-over-role */}

      <div
        style={{
          display: "flex",
          gap: 16,
          padding: "8px 16px",
          borderTop: "1px solid var(--gl-color-border, #f0f0f0)",
          fontSize: 12,
          color: "#999",
        }}
      >
        <span>↑↓ {t("commandPalette.nav", "to navigate")}</span>
        <span>↵ {t("commandPalette.open", "to open")}</span>
        <span>Esc {t("commandPalette.close", "to close")}</span>
      </div>
    </Modal>
  );
}

export default CommandPalette;
