import { describe, it, expect, beforeEach } from "vitest";
import { useThemeStore, DEFAULT_CONFIG } from "./theme-store";

describe("useThemeStore", () => {
  beforeEach(() => {
    localStorage.clear();
    // Reset store to default state for each test
    useThemeStore.setState({
      themeMode: "system",
      resolvedTheme: "light",
      config: { ...DEFAULT_CONFIG },
    });
  });

  it("initializes with default config", () => {
    const state = useThemeStore.getState();
    expect(state.config).toEqual(DEFAULT_CONFIG);
  });

  it("toggles theme from light to dark", () => {
    useThemeStore.setState({ themeMode: "light", resolvedTheme: "light" });
    useThemeStore.getState().toggleTheme();

    const state = useThemeStore.getState();
    expect(state.themeMode).toBe("dark");
  });

  it("toggles theme from dark to system", () => {
    useThemeStore.setState({ themeMode: "dark", resolvedTheme: "dark" });
    useThemeStore.getState().toggleTheme();

    const state = useThemeStore.getState();
    expect(state.themeMode).toBe("system");
  });

  it("toggles theme from system to light", () => {
    useThemeStore.setState({ themeMode: "system", resolvedTheme: "light" });
    useThemeStore.getState().toggleTheme();

    const state = useThemeStore.getState();
    expect(state.themeMode).toBe("light");
  });

  it("sets theme mode to dark directly", () => {
    useThemeStore.getState().setThemeMode("dark");

    const state = useThemeStore.getState();
    expect(state.themeMode).toBe("dark");
    expect(state.resolvedTheme).toBe("dark");
  });

  it("sets theme mode to light directly", () => {
    useThemeStore.getState().setThemeMode("light");

    const state = useThemeStore.getState();
    expect(state.themeMode).toBe("light");
    expect(state.resolvedTheme).toBe("light");
  });

  it("sets theme mode to system and resolves based on matchMedia", () => {
    useThemeStore.getState().setThemeMode("system");

    const state = useThemeStore.getState();
    expect(state.themeMode).toBe("system");
    // In jsdom with matchMedia returning false, system resolves to light
    expect(state.resolvedTheme).toBe("light");
  });

  it("updates config partially with setConfig", () => {
    useThemeStore.getState().setConfig({ primaryColor: "#ff0000" });

    const state = useThemeStore.getState();
    expect(state.config.primaryColor).toBe("#ff0000");
    expect(state.config.borderRadius).toBe(10); // preserved
  });

  it("replaces config entirely with applyConfig", () => {
    useThemeStore.getState().applyConfig({
      primaryColor: "#00ff00",
      darkPrimaryColor: "#00aa00",
      borderRadius: 6,
    });

    const state = useThemeStore.getState();
    expect(state.config.primaryColor).toBe("#00ff00");
    expect(state.config.darkPrimaryColor).toBe("#00aa00");
    expect(state.config.borderRadius).toBe(6);
  });

  it("toggleTheme cycles through light → dark → system → light", () => {
    useThemeStore.setState({ themeMode: "light", resolvedTheme: "light" });

    useThemeStore.getState().toggleTheme();
    expect(useThemeStore.getState().themeMode).toBe("dark");

    useThemeStore.getState().toggleTheme();
    expect(useThemeStore.getState().themeMode).toBe("system");

    useThemeStore.getState().toggleTheme();
    expect(useThemeStore.getState().themeMode).toBe("light");
  });

  it("setConfig preserves other config properties", () => {
    useThemeStore.getState().setConfig({ borderRadius: 4 });

    const state = useThemeStore.getState();
    expect(state.config.borderRadius).toBe(4);
    expect(state.config.primaryColor).toBe(DEFAULT_CONFIG.primaryColor); // preserved
    expect(state.config.darkPrimaryColor).toBe(DEFAULT_CONFIG.darkPrimaryColor); // preserved
  });

  it("applyConfig replaces all config properties", () => {
    useThemeStore.getState().applyConfig({
      primaryColor: "#111111",
      darkPrimaryColor: "#222222",
      borderRadius: 2,
    });

    const state = useThemeStore.getState();
    expect(state.config).toEqual({
      primaryColor: "#111111",
      darkPrimaryColor: "#222222",
      borderRadius: 2,
    });
  });

  it("persists theme mode to localStorage on setThemeMode", () => {
    useThemeStore.getState().setThemeMode("dark");
    expect(localStorage.getItem("gl-theme-mode")).toBe("dark");
  });

  it("persists config to localStorage on setConfig", () => {
    useThemeStore.getState().setConfig({ borderRadius: 8 });
    const stored = localStorage.getItem("gl-theme-config");
    expect(stored).not.toBeNull();
    expect(JSON.parse(stored!).borderRadius).toBe(8);
  });
});
