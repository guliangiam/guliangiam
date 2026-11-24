import type { CSSProperties } from "react";

// Palette: indigo → violet → cyan
const AI_COLORS = ["#6366F1", "#A855F7", "#06B6D4"];

/**
 * AiDots – animated indicator used standalone or as antd Spin indicator.
 */
export const AiDots = ({ size = "medium" }: { size?: "small" | "medium" | "large" }) => {
  const isSmall = size === "small";
  const isLarge = size === "large";

  const dotPx = isSmall ? 5 : isLarge ? 12 : 9;
  const gapPx = isSmall ? 5 : isLarge ? 10 : 8;
  const glowBlur = isSmall ? 0 : isLarge ? 12 : 8;

  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: gapPx }}>
      {AI_COLORS.map((color, i) => (
        <span
          key={i}
          style={{
            display: "inline-block",
            width: dotPx,
            height: dotPx,
            borderRadius: "50%",
            background: color,
            animation: "gl-ai-bounce 1.4s ease-in-out infinite",
            animationDelay: `${i * 0.16}s`,
            boxShadow: glowBlur > 0 ? `0 0 ${glowBlur}px ${color}90` : "none",
          }}
        />
      ))}
    </span>
  );
};

type LoadingType = "page" | "section" | "small";

type LoadingProps = {
  spinning?: boolean;
  tip?: string;
  type?: LoadingType;
  style?: CSSProperties;
};

/**
 * Loading – unified page / section / small loading component.
 *
 * - "page"    – vertically centered in viewport
 * - "section" – padded block for card/content areas (default)
 * - "small"   – inline, tiny dots, no tip
 */
const Loading = ({ spinning = true, tip, type = "section", style }: LoadingProps) => {
  if (!spinning) {
    return null;
  }

  const isPage = type === "page";
  const isSmall = type === "small";

  const wrapperStyle: CSSProperties = {
    display: "flex",
    flexDirection: "column",
    justifyContent: "center",
    alignItems: "center",
    ...(isPage && { width: "100%", height: "calc(100vh - 120px)" }),
    ...(type === "section" && { padding: "48px 0" }),
    ...style,
  };

  const tipStyle: CSSProperties = {
    marginTop: 14,
    fontSize: 13,
    /* WCAG 1.4.3 Contrast — replace #94A3B8 with compliant color (B-04) */
    color: "var(--gl-color-text-secondary, #595959)",
    letterSpacing: "0.05em",
    fontWeight: 400,
    userSelect: "none",
  };

  return (
    <div style={wrapperStyle}>
      <AiDots size={isSmall ? "small" : isPage ? "large" : "medium"} />
      {tip && !isSmall && <div style={tipStyle}>{tip}</div>}
    </div>
  );
};

export default Loading;
