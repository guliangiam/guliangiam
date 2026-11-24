/**
 * 位置型 Scrollspy 的纯函数实现。
 *
 * 旧实现用 IntersectionObserver 把「与观察带相交的分区」按 top 升序取第一个：
 * 当上一个分区很长、仍与观察带相交时，它会把当前分区挤掉，导致点击/滚动后
 * 高亮跳到相邻标签。这里改为「观察线所在的分区」：取最后一个上边缘不高于
 * 观察线的分区；页面滚到底时固定取最后一个分区。
 */

export type ScrollspySection = {
  /** 分区键（与导航 items[].key 一致）。 */
  key: string;
  /** 分区上边缘相对滚动容器内容顶部的偏移。 */
  top: number;
};

/**
 * @param sections 当前可导航分区及其上边缘偏移（顺序不限）。
 * @param bandTop  观察线偏移：sticky header + 顶部导航之下的可视上边界。
 * @param atBottom 滚动容器是否已到底（此时最后一个分区可能永远到不了观察线）。
 * @returns 应高亮的分区键；无分区时返回 undefined。
 */
export function pickActiveSection(
  sections: readonly ScrollspySection[],
  bandTop: number,
  atBottom: boolean,
): string | undefined {
  if (sections.length === 0) return undefined;
  const ordered = [...sections].sort((a, b) => a.top - b.top);
  if (atBottom) return ordered[ordered.length - 1].key;

  let active = ordered[0].key;
  for (const section of ordered) {
    if (section.top > bandTop) break;
    active = section.key;
  }
  return active;
}
