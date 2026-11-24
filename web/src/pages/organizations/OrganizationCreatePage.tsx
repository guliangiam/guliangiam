import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  createOrganization,
  getDeploymentRegions,
  type CreateOrganizationPayload,
} from "@/shared/api/iam/organization";
import { getApplicationOptions } from "@/shared/api/iam";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { queryKeys } from "@/shared/api/query-keys";
import { OrganizationBasicFields } from "./sections/OrganizationBasicFields";
import { OrganizationBrandingFields } from "./sections/OrganizationBrandingFields";
import { OrganizationPasswordFields } from "./sections/OrganizationPasswordFields";
import { OrganizationSecurityFields } from "./sections/OrganizationSecurityFields";

export default function OrganizationCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CreateOrganizationPayload>();

  const regionOptionsQuery = useQuery({
    queryKey: [...queryKeys.organizations.all, "deployment-regions"],
    queryFn: () => getDeploymentRegions(),
  });

  const applicationOptionsQuery = useQuery({
    queryKey: queryKeys.applications.options(),
    queryFn: () => getApplicationOptions(),
  });

  const mutation = useMutation({
    mutationFn: async (values: CreateOrganizationPayload) => {
      return createOrganization(values);
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.organizations.all });
      message.success(tc("createSuccess"));
      navigate(`/organizations/${result.id}`);
    },
  });

  const fieldLabels: Record<string, string> = {
    slug: tc("org.slug"),
    parentId: tr("organization.parentOrganization"),
    regionCode: tr("organization.regionCode"),
    displayName: tc("displayName"),
    status: tc("statusLabel"),
    websiteUrl: tc("org.websiteUrl"),
    logoUrl: tr("organization.logoUrl"),
    faviconUrl: tr("organization.faviconUrl"),
    enableDarkLogo: tr("organization.enableDarkLogo"),
    logoDarkUrl: tr("organization.logoDarkUrl"),
    defaultAvatar: tr("organization.defaultAvatar"),
    defaultApplicationId: tr("organization.defaultApplication"),
    initScore: tr("organization.initScore"),
    passwordType: tr("organization.passwordType"),
    passwordSalt: tr("organization.passwordSalt"),
    passwordObfuscatorType: tr("organization.passwordObfuscatorType"),
    passwordObfuscatorKey: tr("organization.passwordObfuscatorKey"),
    passwordOptions: tr("organization.passwordOptions"),
    defaultPassword: tr("organization.defaultPassword"),
    masterPassword: tr("organization.masterPassword"),
    masterVerificationCode: tr("organization.masterVerificationCode"),
    disableSignin: tr("organization.disableSignin"),
    enableTour: tr("organization.enableTour"),
    useEmailAsUsername: tr("organization.useEmailAsUsername"),
    isProfilePublic: tr("organization.isProfilePublic"),
    hasPrivilegeConsent: tr("organization.hasPrivilegeConsent"),
  };

  return (
    <ResourceFormPage
      title={tr("organization.newOrganization")}
      description={tr("organization.createDescription", {})}
      saving={mutation.isPending}
      form={form}
      fieldLabels={fieldLabels}
      enableDraft
      draftRoute="organizations"
      draftEntityId={"new"}
      onCancel={() => navigate("/organizations")}
      onFinish={async (values: CreateOrganizationPayload) => {
        await mutation.mutateAsync(values);
      }}
    >
      <FormSection
        title={tr("organization.basicSection")}
        description={tr("organization.basicSectionDesc", {})}
        required
      >
        <OrganizationBasicFields
          mode="create"
          regionOptions={regionOptionsQuery.data ?? []}
          regionsError={regionOptionsQuery.isError}
        />
      </FormSection>

      <FormSection
        title={tr("organization.profileSection")}
        description={tr("organization.profileSectionDesc", {})}
        collapsible
        defaultCollapsed={false}
      >
        <OrganizationBrandingFields
          mode="create"
          organizationName=""
          applicationOptions={applicationOptionsQuery.data ?? []}
          applicationsLoading={applicationOptionsQuery.isLoading}
          applicationsError={applicationOptionsQuery.isError}
        />
      </FormSection>

      <FormSection
        title={tr("organization.passwordSection")}
        description={tr("organization.passwordSectionDesc")}
        collapsible
        defaultCollapsed
      >
        <OrganizationPasswordFields mode="create" />
      </FormSection>

      <FormSection
        title={tr("organization.securitySection")}
        description={tr("organization.securitySectionDesc")}
        collapsible
        defaultCollapsed
      >
        <OrganizationSecurityFields mode="create" />
      </FormSection>
    </ResourceFormPage>
  );
}
