import { Alert, Button, Space, Tag, Typography } from "antd";
import { useMemo } from "react";
import { MonacoTextEditor } from "@/shared/ui/MonacoTextEditor";
import { useTranslation } from "react-i18next";
import { t as i18nT } from "i18next";

type CasbinModelEditorProps = {
  value?: string | null;
  onChange?: (value: string) => void;
};

const sectionLabels: Record<string, string> = {
  request_definition: "Request",
  policy_definition: "Policy",
  role_definition: "Role",
  policy_effect: "Effect",
  matchers: "Matchers",
};

const requiredSections = ["request_definition", "policy_definition", "policy_effect", "matchers"];

const templates = {
  RBAC: `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`,
  ACL: `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`,
  ABAC: `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub_rule, obj_rule, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = eval(p.sub_rule) && eval(p.obj_rule) && r.act == p.act
`,
} as const;

function extractSections(modelText: string) {
  return Array.from(modelText.matchAll(/^\[([^\]]+)\]$/gm)).map((match) => match[1].trim());
}

function collectDiagnostics(modelText: string) {
  const sections = extractSections(modelText);
  const diagnostics: string[] = [];

  if (sections.length === 0) {
    diagnostics.push(i18nT("common:casbin.noSectionsDetected"));
    return diagnostics;
  }

  requiredSections.forEach((section) => {
    if (!sections.includes(section)) {
      diagnostics.push(i18nT("common:casbin.missingSection", { section }));
    }
  });

  const duplicates = sections.filter((section, index) => sections.indexOf(section) !== index);
  duplicates.forEach((section) => {
    diagnostics.push(i18nT("common:casbin.duplicateSection", { section }));
  });

  if (!/m\s*=/.test(modelText)) {
    diagnostics.push(i18nT("common:casbin.missingMatchersRule"));
  }

  if (!/e\s*=/.test(modelText)) {
    diagnostics.push(i18nT("common:casbin.missingEffectRule"));
  }

  return diagnostics;
}

export function CasbinModelEditor({ value, onChange }: CasbinModelEditorProps) {
  const { t } = useTranslation("common");
  const modelText = value ?? "";
  const sections = useMemo(() => extractSections(modelText), [modelText]);
  const diagnostics = useMemo(() => collectDiagnostics(modelText), [modelText]);

  return (
    <div className="gl-model-editor">
      <div className="gl-model-editor__toolbar">
        <div>
          <Typography.Text strong>{t("casbin.template")}</Typography.Text>
          <Space wrap>
            {Object.entries(templates).map(([name, template]) => (
              <Button
                key={name}
                size="small"
                onClick={() => {
                  onChange?.(template);
                }}
              >
                {name}
              </Button>
            ))}
          </Space>
        </div>
        <div>
          <Typography.Text strong>{t("casbin.detectedSections")}</Typography.Text>
          <Space wrap>
            {sections.length > 0 ? (
              sections.map((section) => (
                <Tag key={section} color={requiredSections.includes(section) ? "blue" : "default"}>
                  {sectionLabels[section] ?? section}
                </Tag>
              ))
            ) : (
              <Tag>{t("casbin.none")}</Tag>
            )}
          </Space>
        </div>
      </div>

      {diagnostics.length > 0 ? (
        <Alert
          className="gl-model-editor__alert"
          type="warning"
          showIcon
          title={t("casbin.validationTitle")}
          description={
            <ul className="gl-model-editor__issues">
              {diagnostics.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          }
        />
      ) : (
        <Alert
          className="gl-model-editor__alert"
          type="success"
          showIcon
          title={t("casbin.completeTitle")}
          description={t("casbin.completeDescription")}
        />
      )}

      <MonacoTextEditor
        height={500}
        language="ini"
        theme="vs-dark"
        value={modelText}
        onChange={(newValue) => {
          onChange?.(newValue);
        }}
        options={{
          minimap: { enabled: false },
          lineNumbers: "on",
          scrollBeyondLastLine: false,
          wordWrap: "on",
          fontSize: 13,
          tabSize: 2,
          automaticLayout: true,
        }}
        fallback={
          <textarea
            className="gl-model-editor__textarea"
            spellCheck={false}
            value={modelText}
            onChange={(event) => {
              onChange?.(event.target.value);
            }}
          />
        }
      />
    </div>
  );
}
