import { useState } from "react";
import { Tour, type TourProps } from "antd";
import { useTranslation } from "react-i18next";

const TOUR_COMPLETED_KEY = "guliangiam-tour-completed";

type GuidedTourProps = {
  tourId: string;
  steps: TourProps["steps"];
};

/** True when the tour has not been marked as completed in local storage. */
function shouldAutoOpenTour(tourId: string) {
  try {
    const completed = localStorage.getItem(TOUR_COMPLETED_KEY);
    if (!completed) return true;
    const parsed = JSON.parse(completed) as string[];
    return !parsed.includes(tourId);
  } catch {
    return true;
  }
}

/** Tour 步骤指示器 — 只依赖入参，提到模块作用域以避免每次渲染新建组件。 */
function renderTourIndicators(current: number, count: number) {
  return (
    <span style={{ fontSize: 12, color: "var(--gl-color-text-secondary)" }}>
      {current + 1} / {count}
    </span>
  );
}

export function GuidedTour({ tourId, steps }: GuidedTourProps) {
  const { t: _t } = useTranslation("common");
  // The persisted "already seen" flag lives outside React, so it is read once
  // per tour id and re-applied during render when the id changes.
  const [seenTourId, setSeenTourId] = useState(tourId);
  const [open, setOpen] = useState(() => shouldAutoOpenTour(tourId));

  if (tourId !== seenTourId) {
    setSeenTourId(tourId);
    setOpen(shouldAutoOpenTour(tourId));
  }

  const handleClose = () => {
    setOpen(false);
    try {
      const completed = localStorage.getItem(TOUR_COMPLETED_KEY);
      const parsed = completed ? (JSON.parse(completed) as string[]) : [];
      if (!parsed.includes(tourId)) {
        parsed.push(tourId);
      }
      localStorage.setItem(TOUR_COMPLETED_KEY, JSON.stringify(parsed));
    } catch {
      // ignore
    }
  };

  return (
    <Tour
      open={open}
      onClose={handleClose}
      onFinish={handleClose}
      steps={steps}
      indicatorsRender={renderTourIndicators}
    />
  );
}

// oxlint-disable-next-line react/only-export-components -- 重置函数与 GuidedTour 共用 TOUR_COMPLETED_KEY，拆分需改动 GuidedTour/index.ts 的导出方
export function resetAllTours() {
  localStorage.removeItem(TOUR_COMPLETED_KEY);
}

// oxlint-disable-next-line react/only-export-components -- 同上，与 GuidedTour 共享 localStorage key 的存储层
export function resetTour(tourId: string) {
  try {
    const completed = localStorage.getItem(TOUR_COMPLETED_KEY);
    if (!completed) return;
    const parsed = JSON.parse(completed) as string[];
    const filtered = parsed.filter((id) => id !== tourId);
    localStorage.setItem(TOUR_COMPLETED_KEY, JSON.stringify(filtered));
  } catch {
    // ignore
  }
}
