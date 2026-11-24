import { create } from "zustand";
import { persist } from "zustand/middleware";

type LayoutMode = "sidebar" | "top";

type LayoutState = {
  layoutMode: LayoutMode;
  sidebarCollapsed: boolean;
  setLayoutMode: (mode: LayoutMode) => void;
  toggleLayout: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;
  toggleSidebar: () => void;
};

export type { LayoutMode };

export const useLayoutStore = create<LayoutState>()(
  persist(
    (set, get) => ({
      layoutMode: "sidebar",
      sidebarCollapsed: false,
      setLayoutMode: (mode) => set({ layoutMode: mode }),
      toggleLayout: () => {
        const next = get().layoutMode === "sidebar" ? "top" : "sidebar";
        set({ layoutMode: next });
      },
      setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),
      toggleSidebar: () => set({ sidebarCollapsed: !get().sidebarCollapsed }),
    }),
    {
      name: "guliangiam-layout",
      partialize: (state) => ({
        layoutMode: state.layoutMode,
        sidebarCollapsed: state.sidebarCollapsed,
      }),
    },
  ),
);
