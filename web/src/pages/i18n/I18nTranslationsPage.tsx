import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Table, Card, Typography, Select, Input, Button, message, Space } from "antd";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { iamGet, iamUpdate } from "@/shared/api/iam/i18n";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";

import { I18N_TRANSLATIONS } from "@/shared/api/paths";

const { Title } = Typography;
const { Search } = Input;

const LANGUAGE_CODES = [
  "zh-CN",
  "zh-TW",
  "en",
  "ru",
  "ja",
  "ko",
  "de",
  "fr",
  "es",
  "pt",
  "vi",
  "pl",
  "tr",
  "uk",
];

const NATIVE_NAMES: Record<string, string> = {
  "zh-CN": "中文（简体）",
  "zh-TW": "繁體中文",
  ru: "Русский",
  en: "English",
  ko: "한국어",
  de: "Deutsch",
  fr: "Français",
  es: "Español",
  pt: "Português",
  vi: "Tiếng Việt",
  pl: "Polski",
  tr: "Türkçe",
  uk: "Українська",
};

const I18N_KEYS: Record<string, string> = {
  "zh-CN": "common:lang.zhCN",
  "zh-TW": "common:lang.zhTW",
  ru: "common:lang.ru",
  en: "common:lang.en",
  ko: "common:lang.ko",
  de: "common:lang.de",
  fr: "common:lang.fr",
  es: "common:lang.es",
  pt: "common:lang.pt",
  vi: "common:lang.vi",
  pl: "common:lang.pl",
  tr: "common:lang.tr",
  uk: "common:lang.uk",
  ja: "common:lang.ja",
};

export default function I18nTranslationsPage() {
  return (
    <PermissionGuard action="i18n:read">
      <I18nTranslationsWorkspace />
    </PermissionGuard>
  );
}

function I18nTranslationsWorkspace() {
  const { t } = useTranslation();

  const SUPPORTED_LANGUAGES = LANGUAGE_CODES.map((code) => ({
    value: code,
    label: I18N_KEYS[code] ? t(I18N_KEYS[code]) : NATIVE_NAMES[code],
  }));
  const queryClient = useQueryClient();
  const [language, setLanguage] = useState("en");
  const [searchText, setSearchText] = useState("");
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editingValue, setEditingValue] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["i18n-translations", language],
    queryFn: () => iamGet<Record<string, string>>(`${I18N_TRANSLATIONS(language)}`),
  });

  const updateMutation = useMutation({
    mutationFn: (payload: Record<string, string>) =>
      iamUpdate(`${I18N_TRANSLATIONS(language)}`, payload),
    onSuccess: () => {
      message.success(t("common:success", "Updated successfully"));
      queryClient.invalidateQueries({ queryKey: ["i18n-translations", language] });
      setEditingKey(null);
    },
    onError: () => {
      message.error(t("common:error", "Update failed"));
    },
  });

  const translations = data || {};
  const filteredEntries = Object.entries(translations).filter(
    ([key, value]) =>
      !searchText ||
      key.toLowerCase().includes(searchText.toLowerCase()) ||
      value.toLowerCase().includes(searchText.toLowerCase()),
  );

  const columns = [
    {
      title: "Key",
      dataIndex: "key",
      key: "key",
      width: "40%",
      render: (key: string) => <code style={{ fontSize: 12 }}>{key}</code>,
    },
    {
      title: t("common:translation", "Translation"),
      dataIndex: "value",
      key: "value",
      width: "60%",
      render: (value: string, record: { key: string }) => {
        if (editingKey === record.key) {
          return (
            <Space>
              <Input
                value={editingValue}
                onChange={(e) => setEditingValue(e.target.value)}
                size="small"
                style={{ width: 300 }}
              />
              <Button
                type="primary"
                size="small"
                loading={updateMutation.isPending}
                onClick={() => {
                  updateMutation.mutate({ [record.key]: editingValue });
                }}
              >
                {t("common:save", "Save")}
              </Button>
              <Button
                size="small"
                disabled={updateMutation.isPending}
                onClick={() => setEditingKey(null)}
              >
                {t("common:cancel", "Cancel")}
              </Button>
            </Space>
          );
        }
        return (
          <PermissionGuard action="i18n:write" fallback={<span>{value}</span>}>
            <button
              type="button"
              style={{
                cursor: "pointer",
                background: "none",
                border: 0,
                padding: 0,
                font: "inherit",
                color: "inherit",
              }}
              onClick={() => {
                setEditingKey(record.key);
                setEditingValue(value);
              }}
            >
              {value}
            </button>
          </PermissionGuard>
        );
      },
    },
  ];

  const tableData = filteredEntries.map(([key, value]) => ({ key, value }));

  return (
    <div className="gl-page">
      <Card>
        <Space orientation="vertical" style={{ width: "100%" }} size="middle">
          <Title level={4}>{t("common:translations", "Translations")}</Title>
          <Space wrap>
            <Select
              value={language}
              onChange={setLanguage}
              aria-label={t("common:language", "Language")}
              options={SUPPORTED_LANGUAGES}
              style={{ width: 180 }}
            />
            <Search
              placeholder={t("common:search", "Search")}
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              style={{ width: 300 }}
              allowClear
            />
          </Space>
          <Table
            dataSource={tableData}
            columns={columns}
            rowKey="key"
            loading={isLoading}
            pagination={{ pageSize: 50 }}
            size="small"
          />
        </Space>
      </Card>
    </div>
  );
}
