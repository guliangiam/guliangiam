import { useEffect } from "react";
import { useTranslation } from "react-i18next";

/**
 * useAccessibleTableRegions — 让横向滚动的表格容器可键盘聚焦。
 *
 * antd Table 的 `.ant-table-content` / `.ant-table-body` 在内容超宽时是
 * `overflow: auto` 的滚动区，但没有 tabindex，键盘用户无法进入滚动（axe
 * `scrollable-region-focusable`，serious）。全站表格分散在页面内联 Table 与
 * ResourceListCard 两条路径，因此在这里统一补 `tabindex=0` + `role=region` +
 * 名称，并用 MutationObserver 覆盖后续挂载（分页/筛选重渲染、懒加载页面）。
 *
 * 只处理实际滚动样式为 auto/scroll 的容器，不改变布局，也不重复标注。
 */
export function useAccessibleTableRegions() {
  const { t } = useTranslation("common");

  useEffect(() => {
    const label = t("table", { defaultValue: "Data table" });
    let frame = 0;

    const patch = () => {
      for (const region of document.querySelectorAll<HTMLElement>(
        ".ant-table-content, .ant-table-body",
      )) {
        if (region.dataset.a11yRegion === "true") continue;
        const style = getComputedStyle(region);
        if (!/(auto|scroll)/.test(`${style.overflowX} ${style.overflowY}`)) continue;
        region.tabIndex = 0;
        region.setAttribute("role", "region");
        if (!region.getAttribute("aria-label")) region.setAttribute("aria-label", label);
        region.dataset.a11yRegion = "true";
      }
    };

    const schedule = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(() => {
        frame = 0;
        patch();
      });
    };

    patch();
    const observer = new MutationObserver(schedule);
    observer.observe(document.body, { childList: true, subtree: true });

    return () => {
      observer.disconnect();
      if (frame) window.cancelAnimationFrame(frame);
    };
  }, [t]);
}
