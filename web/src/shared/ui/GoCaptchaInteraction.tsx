import { useRef, useState } from "react";
import { Button, Space, Typography } from "antd";
import { useTranslation } from "react-i18next";

/**
 * go_captcha 本地图形验证码交互组件（provider_type="go_captcha"）。
 *
 * 两种形态与后端 `internal/infra/captcha/go_captcha_provider.go` 的 token
 * 契约对齐，坐标一律使用相对主图的百分比（0-100），与渲染尺寸无关：
 *   - click: "fx,fy;fx,fy" —— 按下方缩略图提示顺序点击主图中的文字
 *   - slide: "frac"        —— 拖动瓦片，上报瓦片左缘 X 占主图宽度的百分比
 *
 * onChange 上报的是 value 部分（不含 captchaId），与 default 形态用户输入的
 * 验证码字符串同位，由调用方拼接/提交。
 */
export type GoCaptchaInteractionProps = {
  shape?: string | null;
  /** base64 主图（JPEG，无 data: 前缀）。 */
  masterImage?: string | null;
  /** base64 副图：click=提示缩略图条，slide=待拖动瓦片（PNG）。 */
  subImage?: string | null;
  /** antd Form.Item 注入；组件本身非受控，可忽略。 */
  value?: string;
  onChange?: (value: string) => void;
};

const clampPct = (v: number) => Math.min(100, Math.max(0, v));

type ClickPoint = { x: number; y: number };

export function GoCaptchaInteraction({
  shape,
  masterImage,
  subImage,
  onChange,
}: GoCaptchaInteractionProps) {
  const { t } = useTranslation("common");
  const isSlide = shape === "slide";

  // ── click 形态状态 ──
  const [points, setPoints] = useState<ClickPoint[]>([]);
  const masterRef = useRef<HTMLImageElement>(null);

  // ── slide 形态状态 ──
  const containerRef = useRef<HTMLDivElement>(null);
  const [leftPct, setLeftPct] = useState(0);
  const dragRef = useRef<{ startX: number; startLeft: number } | null>(null);

  const handleMasterClick = (e: React.MouseEvent<HTMLButtonElement>) => {
    const rect = masterRef.current?.getBoundingClientRect();
    if (!rect || rect.width === 0 || rect.height === 0) return;
    const next = [
      ...points,
      {
        x: clampPct(((e.clientX - rect.left) / rect.width) * 100),
        y: clampPct(((e.clientY - rect.top) / rect.height) * 100),
      },
    ];
    setPoints(next);
    onChange?.(next.map((p) => `${p.x.toFixed(4)},${p.y.toFixed(4)}`).join(";"));
  };

  const handleTilePointerDown = (e: React.PointerEvent<HTMLImageElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId);
    dragRef.current = { startX: e.clientX, startLeft: leftPct };
  };

  const handleTilePointerMove = (e: React.PointerEvent<HTMLImageElement>) => {
    const drag = dragRef.current;
    const container = containerRef.current;
    if (!drag || !container) return;
    const rect = container.getBoundingClientRect();
    if (rect.width === 0) return;
    const next = clampPct(drag.startLeft + ((e.clientX - drag.startX) / rect.width) * 100);
    setLeftPct(next);
    onChange?.(next.toFixed(4));
  };

  const handleTilePointerUp = () => {
    dragRef.current = null;
  };

  const handleReset = () => {
    if (isSlide) {
      setLeftPct(0);
    } else {
      setPoints([]);
    }
    onChange?.("");
  };

  if (!masterImage) {
    return null;
  }

  return (
    <Space orientation="vertical" size={8} style={{ width: "100%" }}>
      {isSlide ? (
        <div
          ref={containerRef}
          style={{ position: "relative", width: 300, maxWidth: "100%", lineHeight: 0 }}
        >
          <img
            src={`data:image/jpeg;base64,${masterImage}`}
            alt="captcha"
            draggable={false}
            style={{ width: "100%", display: "block", borderRadius: 4 }}
          />
          {subImage ? (
            <img
              src={`data:image/png;base64,${subImage}`}
              alt="captcha tile"
              draggable={false}
              onPointerDown={handleTilePointerDown}
              onPointerMove={handleTilePointerMove}
              onPointerUp={handleTilePointerUp}
              onPointerCancel={handleTilePointerUp}
              style={{
                position: "absolute",
                top: 0,
                left: `${leftPct}%`,
                height: "100%",
                cursor: "grab",
                touchAction: "none",
                userSelect: "none",
              }}
            />
          ) : null}
        </div>
      ) : (
        <>
          <div style={{ position: "relative", width: 300, maxWidth: "100%", lineHeight: 0 }}>
            <button
              type="button"
              onClick={handleMasterClick}
              aria-label={t("captcha.clickHint")}
              style={{
                border: 0,
                padding: 0,
                background: "transparent",
                display: "block",
                width: "100%",
                cursor: "crosshair",
              }}
            >
              <img
                ref={masterRef}
                src={`data:image/jpeg;base64,${masterImage}`}
                alt="captcha"
                draggable={false}
                style={{ width: "100%", display: "block", borderRadius: 4 }}
              />
            </button>
            {points.map((p, i) => (
              <span
                key={i}
                style={{
                  position: "absolute",
                  left: `${p.x}%`,
                  top: `${p.y}%`,
                  transform: "translate(-50%, -50%)",
                  width: 18,
                  height: 18,
                  borderRadius: "50%",
                  background: "rgba(22, 119, 255, 0.75)",
                  color: "#fff",
                  fontSize: 12,
                  lineHeight: "18px",
                  textAlign: "center",
                  pointerEvents: "none",
                }}
              >
                {i + 1}
              </span>
            ))}
          </div>
          {subImage ? (
            <img
              src={`data:image/png;base64,${subImage}`}
              alt="captcha hint"
              style={{ height: 36, display: "block" }}
            />
          ) : null}
        </>
      )}
      <Space size={8}>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {isSlide ? t("captcha.dragHint") : t("captcha.clickHint")}
        </Typography.Text>
        <Button size="small" type="text" onClick={handleReset}>
          {t("reset")}
        </Button>
      </Space>
    </Space>
  );
}
