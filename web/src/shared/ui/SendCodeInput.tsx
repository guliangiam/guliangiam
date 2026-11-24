import { useState, useCallback, useRef, useEffect } from "react";
import { Button, Input, Space } from "antd";

type SendCodeInputProps = {
  value?: string;
  onChange?: (value: string) => void;
  onSendCode: () => Promise<void>;
  cooldown?: number;
  placeholder?: string;
  sendButtonText?: string;
  disabled?: boolean;
  /** 仅禁用发送按钮（输入框仍可填写）：收件人未就绪时阻止发码而不锁定输入。 */
  sendDisabled?: boolean;
  /** 透传到内部 Input 的 autoComplete（验证码字段建议 "one-time-code" 以启用短信自动填充）。 */
  autoComplete?: string;
};

export function SendCodeInput({
  value,
  onChange,
  onSendCode,
  cooldown = 60,
  placeholder = "Enter verification code",
  sendButtonText = "Send Code",
  disabled = false,
  sendDisabled = false,
  autoComplete,
}: SendCodeInputProps) {
  const [leftTime, setLeftTime] = useState(0);
  const [sending, setSending] = useState(false);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Clean up the interval on unmount.
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, []);

  const handleSend = useCallback(async () => {
    if (leftTime > 0 || sending) return;
    setSending(true);
    try {
      await onSendCode();
      let remaining = cooldown;
      setLeftTime(remaining);
      timerRef.current = setInterval(() => {
        remaining--;
        setLeftTime(remaining);
        if (remaining <= 0) {
          if (timerRef.current) {
            clearInterval(timerRef.current);
            timerRef.current = null;
          }
        }
      }, 1000);
    } finally {
      setSending(false);
    }
    // oxlint-disable-next-line react/memo-dependencies -- onSendCode / cooldown 在 try 块内被读取，oxlint 的依赖分析不进入 try，属误报
  }, [leftTime, sending, onSendCode, cooldown]);

  return (
    <Space.Compact style={{ width: "100%" }}>
      <Input
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete={autoComplete}
      />
      <Button
        onClick={handleSend}
        loading={sending}
        disabled={disabled || sendDisabled || leftTime > 0}
        style={{ whiteSpace: "nowrap" }}
      >
        {leftTime > 0 ? `${leftTime}s` : sendButtonText}
      </Button>
    </Space.Compact>
  );
}
