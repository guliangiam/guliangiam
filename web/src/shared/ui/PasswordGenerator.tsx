import { ThunderboltOutlined } from "@ant-design/icons";
import { Button, Input, Tooltip } from "antd";
import { memo, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { generateRandomPassword } from "@/shared/lib/random-password";

/**
 * PasswordGenerator — 敏感口令字段的「随机生成」能力（对标 Casdoor
 * OrganizationEditPage 的 defaultPassword 随机按钮并增强）。
 *
 * 纯随机函数在 `@/shared/lib/random-password`，本文件只含组件。
 * - `PasswordGenerateButton`：图标按钮，可用作 Input 的 suffix 或独立操作。
 * - `PasswordGeneratorInput`：受控输入壳，透传 value/onChange 给 antd Form.Item，
 *   生成值经由 onChange 写回，保证草稿 dirty 追踪与校验链路生效。
 */

type PasswordGenerateButtonProps = {
  onGenerated: (password: string) => void;
  length?: number;
};

/** 随机生成图标按钮（可放入 Input suffix 或表单行尾）。 */
export const PasswordGenerateButton = memo(function PasswordGenerateButton({
  onGenerated,
  length,
}: PasswordGenerateButtonProps) {
  const { t } = useTranslation("common");
  return (
    <Tooltip title={t("generateRandom")}>
      <Button
        type="text"
        size="small"
        icon={<ThunderboltOutlined />}
        aria-label={t("generateRandom")}
        onMouseDown={(event) => event.preventDefault()}
        onClick={() => onGenerated(generateRandomPassword(length))}
      />
    </Tooltip>
  );
});

type PasswordGeneratorInputProps = {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  /** 用密文输入框（masterPassword 等），默认明文（defaultPassword）。 */
  masked?: boolean;
  generateLength?: number;
  disabled?: boolean;
};

/**
 * Form.Item 直接子元素形态：透传 value/onChange，尾部带随机生成按钮。
 * 生成结果走 onChange，因此草稿 dirty 追踪与 rules 校验与手输一致。
 */
export const PasswordGeneratorInput = memo(function PasswordGeneratorInput({
  value,
  onChange,
  placeholder,
  masked = false,
  generateLength,
  disabled,
}: PasswordGeneratorInputProps) {
  const handleGenerate = useCallback((password: string) => onChange?.(password), [onChange]);
  const suffix = <PasswordGenerateButton length={generateLength} onGenerated={handleGenerate} />;
  return masked ? (
    <Input.Password
      value={value}
      placeholder={placeholder}
      disabled={disabled}
      onChange={(event) => onChange?.(event.target.value)}
      suffix={suffix}
    />
  ) : (
    <Input
      value={value}
      placeholder={placeholder}
      disabled={disabled}
      onChange={(event) => onChange?.(event.target.value)}
      suffix={suffix}
    />
  );
});
