/**
 * 详情 / 配置页的分区导航壳（副菜单）。
 *
 * 三种落位共享同一份 `children`（分区表单只渲染一次），差异仅在栅格与定位：
 *   - rail-right：内容在左、导航右边缘常驻（桌面默认）
 *   - rail-left ：导航左列、内容在右
 *   - rail-top  ：导航为内容顶部的 sticky 横向条（窄屏强制形态）
 *
 * DOM 顺序恒为「导航在前、内容在后」，落点由 CSS 栅格列位决定，因此切换方案
 * 不重挂载表单。锚点用原生 `<a href="#...">`：禁用脚本也能跳转，键盘 Enter
 * 可触发；Scrollspy 高亮当前分区，动效尊重 `prefers-reduced-motion`。
 */
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { selectResolvedSectionNavScheme, useSectionNavStore } from "./section-nav-store";
import { pickActiveSection, type ScrollspySection } from "./scrollspy";
import type { SectionNavItem, SectionNavScheme } from "./types";

/** 低于该宽度无视偏好强制顶部横向条（方案 C 形态），避免右栏挤压双列表单。 */
const RAIL_TOP_OVERRIDE_QUERY = "(max-width: 1080px)";

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/** 最近的可滚动祖先；没有则回退 window（文档级滚动）。 */
function findScrollParent(element: HTMLElement | null): HTMLElement | null {
  let node = element?.parentElement ?? null;
  while (node) {
    const { overflowY } = getComputedStyle(node);
    if (overflowY === "auto" || overflowY === "scroll") return node;
    node = node.parentElement;
  }
  return null;
}

/** 读取元素上的 px 级 CSS 变量（如 --gl-sticky-header-height）。 */
function readPxVar(element: HTMLElement | null, name: string): number {
  if (!element) return 0;
  const raw = getComputedStyle(element).getPropertyValue(name).trim();
  const match = raw.match(/^(\d+(?:\.\d+)?)px$/);
  return match ? Number(match[1]) : 0;
}

/** 分区上边缘相对滚动容器内容顶部的偏移（无滚动容器时相对文档）。 */
function sectionTopWithin(element: HTMLElement, container: HTMLElement | null): number {
  const rect = element.getBoundingClientRect();
  if (!container) return rect.top + window.scrollY;
  return rect.top - container.getBoundingClientRect().top + container.scrollTop;
}

export type SectionNavShellProps = {
  /** 分区清单：与 `children` 中的分区容器同源，顺序一致。 */
  items: SectionNavItem[];
  /** 分区内容，三种落位共用，禁止按方案复制多份。 */
  children: ReactNode;
  /** 导航区域的 aria-label；缺省取 `common:sectionNav.sectionsAriaLabel`。 */
  ariaLabel?: string;
  /**
   * 当前分区键（如深链 `?tab=<key>` 解析结果）。挂载与后续变化都会把对应
   * 分区滚入视野并临时锁定高亮，直到用户再次主动滚动；传值应命中 `items` 中的某个 key。
   */
  initialActiveKey?: string;
  /**
   * 用户点击分区后回调（此时已完成滚动）。用于把激活分区写回 URL 深链，
   * 例如映射到 `?tab=<key>`；Scrollspy 跟随滚动的高亮不触发该回调。
   */
  onNavigate?: (key: string) => void;
};

export function SectionNavShell({
  items,
  children,
  ariaLabel,
  initialActiveKey,
  onNavigate,
}: SectionNavShellProps) {
  const { t } = useTranslation("common");
  const storedScheme = useSectionNavStore(selectResolvedSectionNavScheme);
  const forceTop = useMediaQuery(RAIL_TOP_OVERRIDE_QUERY);
  const scheme: SectionNavScheme = forceTop ? "rail-top" : storedScheme;

  const [activeKey, setActiveKey] = useState(initialActiveKey ?? items[0]?.key ?? "");

  // 深链/浏览器前进后退是「外部输入改变 prop」：React 推荐在渲染阶段对齐派生
  // 状态，而不是用 effect 再触发一次渲染（避免级联渲染与闪烁）。
  const [syncedInitialKey, setSyncedInitialKey] = useState(initialActiveKey);
  if (initialActiveKey !== syncedInitialKey) {
    setSyncedInitialKey(initialActiveKey);
    if (initialActiveKey) setActiveKey(initialActiveKey);
  }

  const layoutRef = useRef<HTMLDivElement>(null);
  const navRef = useRef<HTMLElement>(null);
  // 页面通常内联构造 items 数组，每次渲染身份都会变化。用 ref 保存最新清单，
  // 再派生稳定签名供 effect 依赖，避免表单输入等无关重渲染时反复解绑监听、
  // 甚至重复触发深链滚动把页面拽回分区顶部。
  const itemsRef = useRef(items);
  useEffect(() => {
    itemsRef.current = items;
  });
  const itemsKey = items
    .map((item) => `${item.key}:${item.anchor}:${item.disabled ? 1 : 0}`)
    .join("|");
  // 点击/深链触发的程序化滚动期间锁定高亮：Scrollspy 不再改写，避免平滑滚动
  // 经过中间分区时高亮逐段跳变；用户一旦主动滚动（wheel/touch/键盘/拖动滚动条）
  // 立即解除锁定，交还给位置判定。
  const clickLockRef = useRef<string | null>(null);

  // 动态测量 section nav 高度：写入 ——gl-section-nav-height，
  // 供锚点 scroll-margin-top / Scrollspy 观察线实时计算。
  // 仅 rail-top（横向条 sticky 在 header 正下方）需要计入纵向偏移；
  // rail-right / rail-left 的导航在侧边，纵向偏移必须写 0px，
  // 否则锚点会跳过头，落到视口过低的错误位置。
  useEffect(() => {
    const nav = navRef.current;
    const layout = layoutRef.current;
    if (!nav || !layout) return;

    const writeHeight = () => {
      // 形态切换后必须按当前 scheme 重写：rail-top 写实测高度，侧栏形态写 0px。
      if (scheme !== "rail-top") {
        layout.style.setProperty("--gl-section-nav-height", "0px");
      } else {
        const h = nav.getBoundingClientRect().height;
        layout.style.setProperty("--gl-section-nav-height", `${Math.ceil(h)}px`);
      }
      // 观察线依赖导航高度：高度落定后通知 Scrollspy 立即重算一次高亮。
      window.dispatchEvent(new Event("gl-section-nav-updated"));
    };

    const raf = requestAnimationFrame(writeHeight);
    const ro = new ResizeObserver(writeHeight);
    ro.observe(nav);

    // rail-top 形态下横向滚动条出现/消失也会改变高度
    const mo = new MutationObserver(writeHeight);
    mo.observe(nav, { childList: true, subtree: true, characterData: true });

    const onResize = () => writeHeight();
    window.addEventListener("resize", onResize);

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      mo.disconnect();
      window.removeEventListener("resize", onResize);
    };
  }, [scheme]);

  // Scrollspy：按「观察线所在分区」判定高亮。
  // 观察线 = 滚动位置 + sticky header + 顶部导航高度；取最后一个上边缘不高于
  // 观察线的分区。相比 IntersectionObserver，长分区不会再把下一个分区挤掉，
  // 也不会在平滑滚动途中把高亮切到相邻标签。
  useEffect(() => {
    if (!itemsKey) return;
    const layout = layoutRef.current;
    if (!layout) return;
    const container = findScrollParent(layout);

    const compute = () => {
      if (clickLockRef.current) return;
      const sections: ScrollspySection[] = [];
      for (const item of itemsRef.current) {
        if (item.disabled) continue;
        const element = document.getElementById(item.anchor);
        if (!element) continue;
        sections.push({ key: item.key, top: sectionTopWithin(element, container) });
      }
      if (sections.length === 0) return;

      const scrollTop = container ? container.scrollTop : window.scrollY;
      const viewportHeight = container ? container.clientHeight : window.innerHeight;
      const scrollHeight = container
        ? container.scrollHeight
        : document.documentElement.scrollHeight;
      const atBottom = scrollTop + viewportHeight >= scrollHeight - 2;
      const card = layout.closest<HTMLElement>(".gl-resource-card");
      const headerH = readPxVar(card ?? layout, "--gl-sticky-header-height");
      const navH = readPxVar(layout, "--gl-section-nav-height");
      const next = pickActiveSection(sections, scrollTop + headerH + navH + 1, atBottom);
      if (next) setActiveKey((prev) => (prev === next ? prev : next));
    };

    let frame = 0;
    const onScroll = () => {
      if (clickLockRef.current || frame) return;
      frame = requestAnimationFrame(() => {
        frame = 0;
        compute();
      });
    };
    const releaseLock = () => {
      if (!clickLockRef.current) return;
      clickLockRef.current = null;
      onScroll();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (["PageUp", "PageDown", "ArrowUp", "ArrowDown", "Home", "End", " "].includes(event.key)) {
        releaseLock();
      }
    };

    const initialFrame = requestAnimationFrame(compute);
    const scrollTarget: HTMLElement | Window = container ?? window;
    scrollTarget.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    window.addEventListener("wheel", releaseLock, { passive: true });
    window.addEventListener("touchstart", releaseLock, { passive: true });
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("pointerdown", releaseLock);
    window.addEventListener("gl-section-nav-updated", onScroll);

    return () => {
      cancelAnimationFrame(initialFrame);
      if (frame) cancelAnimationFrame(frame);
      scrollTarget.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      window.removeEventListener("wheel", releaseLock);
      window.removeEventListener("touchstart", releaseLock);
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("pointerdown", releaseLock);
      window.removeEventListener("gl-section-nav-updated", onScroll);
    };
  }, [itemsKey]);

  // 深链/浏览器前进后退：地址栏分区变化后等一次布局测量再滚动，并锁定高亮，
  // 保证落点与标签一致（页面自身不再额外调用 scrollIntoView）。
  // 高亮已在渲染阶段对齐，这里只做程序化滚动。
  useEffect(() => {
    if (!initialActiveKey || !itemsKey) return;
    const item = itemsRef.current.find(
      (entry) => entry.key === initialActiveKey && !entry.disabled,
    );
    if (!item) return;
    clickLockRef.current = item.key;

    let inner = 0;
    const outer = requestAnimationFrame(() => {
      inner = requestAnimationFrame(() => {
        document.getElementById(item.anchor)?.scrollIntoView?.({ block: "start" });
      });
    });
    return () => {
      cancelAnimationFrame(outer);
      if (inner) cancelAnimationFrame(inner);
    };
  }, [initialActiveKey, itemsKey]);

  if (items.length === 0) return <>{children}</>;

  return (
    <div ref={layoutRef} className={`gl-section-nav-layout gl-section-nav-layout--${scheme}`}>
      <nav
        ref={navRef}
        className="gl-section-nav"
        aria-label={ariaLabel ?? t("sectionNav.sectionsAriaLabel", { defaultValue: "Sections" })}
      >
        <div className="gl-section-nav__list">
          {items.map((item) => {
            const active = !item.disabled && item.key === activeKey;
            return (
              <a
                key={item.key}
                href={`#${item.anchor}`}
                aria-current={active ? "location" : undefined}
                aria-disabled={item.disabled ? "true" : undefined}
                tabIndex={item.disabled ? -1 : undefined}
                className="gl-section-nav__link"
                style={item.disabled ? { opacity: 0.45, cursor: "not-allowed" } : undefined}
                onClick={(event) => {
                  event.preventDefault();
                  if (item.disabled) return;
                  clickLockRef.current = item.key;
                  setActiveKey(item.key);
                  document.getElementById(item.anchor)?.scrollIntoView?.({
                    behavior: prefersReducedMotion() ? "auto" : "smooth",
                    block: "start",
                  });
                  onNavigate?.(item.key);
                }}
              >
                <span>{item.label}</span>
              </a>
            );
          })}
        </div>
      </nav>
      <div className="gl-section-nav__content">{children}</div>
    </div>
  );
}
