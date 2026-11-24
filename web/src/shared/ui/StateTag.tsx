import { Tag } from "antd";
import { memo } from "react";
import { useTranslation } from "react-i18next";
import { stateCaptionKey, stateColor } from "@/shared/constants/enums";

export type StateTagProps = {
  /** Lifecycle value exactly as the server sent it, e.g. `payment_pending`. */
  value: string | null | undefined;
};

/**
 * The one place a lifecycle value becomes a coloured tag.
 *
 * Every commerce page used to carry its own `stateColorMap` keyed by the
 * PascalCase spelling of the day, so the same state had one colour in the list
 * and another in the detail view, and the values in the map named states the
 * server had stopped sending. Colour and caption both come from the registry
 * set now, which means a page cannot hold a private copy of the vocabulary.
 *
 * The read side is open (手册 4.14.3 item 2): a value this build does not know
 * still renders, in the neutral colour, under its raw name. Hiding it would be
 * worse than showing it — the operator would see an empty cell for a state the
 * server considers real. `scripts/check-enums.py` proves every *declared* value
 * has a caption in every language pack, so the raw-name path is reserved for a
 * deployed console that is behind the server.
 */
export const StateTag = memo(function StateTag({ value }: StateTagProps) {
  const { t } = useTranslation("common");
  if (!value) return <>-</>;
  return <Tag color={stateColor(value)}>{t(stateCaptionKey(value), { defaultValue: value })}</Tag>;
});
