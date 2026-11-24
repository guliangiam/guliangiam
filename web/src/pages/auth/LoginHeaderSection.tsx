import type { ReactNode } from "react";
import { Space } from "antd";
import { isSigninItemShown, type SigninItemConfig } from "@/shared/lib/auth-signin-items";
import { HeaderSigninItem } from "./LoginAuxiliarySigninItems";

type LoginHeaderSectionProps = {
  availableLanguages: string[];
  headerSigninItems: SigninItemConfig[];
  logoAlt: string;
  logoUrl?: string | null;
  onBack: () => void;
  onChangeLanguage: (nextLanguage: string) => void;
  preferredLanguage: string;
  renderSigninItemCss: (item: { customCss?: string | null }) => ReactNode;
  resolvedApplicationName: string;
  resolvedApplicationOrganization?: string | null;
};

export function LoginHeaderSection(props: LoginHeaderSectionProps) {
  const {
    availableLanguages,
    headerSigninItems,
    logoAlt,
    logoUrl,
    onBack,
    onChangeLanguage,
    preferredLanguage,
    renderSigninItemCss,
    resolvedApplicationName,
    resolvedApplicationOrganization,
  } = props;

  return (
    <Space orientation="vertical" size={12} style={{ width: "100%", marginBottom: 12 }}>
      {headerSigninItems.map((item, index) => {
        if (!isSigninItemShown(item)) {
          return null;
        }

        const itemKey = `${resolvedApplicationOrganization ?? "admin"}_${resolvedApplicationName}_${item.name ?? "header"}_${index}`;

        return (
          <HeaderSigninItem
            availableLanguages={availableLanguages}
            item={item}
            itemKey={itemKey}
            key={itemKey}
            logoAlt={logoAlt}
            logoUrl={logoUrl}
            onBack={onBack}
            onChangeLanguage={onChangeLanguage}
            preferredLanguage={preferredLanguage}
            renderSigninItemCss={renderSigninItemCss}
          />
        );
      })}
    </Space>
  );
}
