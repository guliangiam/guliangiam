import { useQuery } from "@tanstack/react-query";
import { Table, Tag, Card, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { iamList } from "@/shared/api/iam/i18n";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";

import { I18N_LANGUAGES } from "@/shared/api/paths";

const { Title } = Typography;

interface Language {
  code: string;
  name: string;
  nativeName: string;
}

export default function I18nLanguagesPage() {
  return (
    <PermissionGuard action="i18n:read">
      <I18nLanguagesWorkspace />
    </PermissionGuard>
  );
}

function I18nLanguagesWorkspace() {
  const { t } = useTranslation();

  const { data, isLoading } = useQuery({
    queryKey: ["i18n-languages"],
    queryFn: () => iamList<Language>(I18N_LANGUAGES),
  });

  const columns = [
    {
      title: t("common:code", "Code"),
      dataIndex: "code",
      key: "code",
      render: (code: string) => <Tag color="blue">{code}</Tag>,
    },
    {
      title: t("common:name", "Name"),
      dataIndex: "name",
      key: "name",
    },
    {
      title: t("common:nativeName", "Native Name"),
      dataIndex: "nativeName",
      key: "nativeName",
    },
  ];

  return (
    <div className="gl-page">
      <Card>
        <Title level={4}>{t("common:languages", "Languages")}</Title>
        <Table
          dataSource={data || []}
          columns={columns}
          rowKey="code"
          loading={isLoading}
          pagination={false}
        />
      </Card>
    </div>
  );
}
