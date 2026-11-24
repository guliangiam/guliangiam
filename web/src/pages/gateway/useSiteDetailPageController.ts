import { App, Form } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  getGatewaySites,
  getGatewaySite,
  updateGatewaySite,
  deleteGatewaySite,
  provisionSiteCert,
  renewSiteCert,
  getSiteCertStatus,
  checkSiteHealth,
  getSiteHealth,
} from "@/shared/api/iam/gateway";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

export type SiteFormValues = {
  name: string;
  host: string;
  status: string;
  sslMode?: string;
  forceHttps?: boolean;
  enableHealthCheck?: boolean;
  healthCheckInterval?: number;
  enableAlert?: boolean;
  alertInterval?: number;
  alertTryTimes?: number;
};

export function useSiteDetailPageController() {
  const { organizationName, siteName } = useParams();
  const account = useSessionStore((state) => state.account);
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<SiteFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const listQuery = useQuery({
    queryKey: ["gateway-sites", "auth"],
    queryFn: () => getGatewaySites(),
    enabled: account !== null,
  });

  const matchedSite = listQuery.data?.find(
    (s) => s.organizationId === organizationName && s.name === siteName,
  );

  const detailQuery = useQuery({
    queryKey: ["gateway-site", "auth", "detail", matchedSite?.id],
    queryFn: () => getGatewaySite(matchedSite!.id),
    enabled: !!matchedSite?.id,
  });

  const site = detailQuery.data;
  const siteId = matchedSite?.id ?? "";

  const certQuery = useQuery({
    queryKey: ["site-cert", siteId],
    queryFn: () => getSiteCertStatus(siteId),
    enabled: !!siteId,
  });

  const healthQuery = useQuery({
    queryKey: ["site-health", siteId],
    queryFn: () => getSiteHealth(siteId),
    enabled: !!siteId,
  });

  const updateMutation = useMutation({
    mutationFn: async (values: SiteFormValues) => {
      const current = site!;
      // 后端 UpdateSite 为全量替换：未随表单提交的字段（证书绑定、告警渠道、
      // metadata）必须回传当前值，否则任意保存都会清空它们。
      return updateGatewaySite(matchedSite!.id, {
        name: values.name,
        host: values.host,
        status: values.status,
        sslMode: values.sslMode ?? current.sslMode,
        certId: current.certId,
        forceHttps: values.forceHttps ?? current.forceHttps,
        enableHealthCheck: values.enableHealthCheck ?? current.enableHealthCheck,
        healthCheckInterval: values.healthCheckInterval ?? current.healthCheckInterval,
        enableAlert: values.enableAlert ?? current.enableAlert,
        alertInterval: values.alertInterval ?? current.alertInterval,
        alertTryTimes: values.alertTryTimes ?? current.alertTryTimes,
        alertProviderIds: current.alertProviderIds,
        metadata: current.metadata ?? undefined,
        revision: current.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      queryClient.invalidateQueries({ queryKey: ["gateway-sites"] });
      // 详情键是单数 ["gateway-site",…]，不失效它 revision 陈旧 → 二次保存 409。
      queryClient.invalidateQueries({ queryKey: ["gateway-site"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteGatewaySite(matchedSite!.id),
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      queryClient.invalidateQueries({ queryKey: ["gateway-sites"] });
      navigate("/sites");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const provisionCertMutation = useMutation({
    mutationFn: () => provisionSiteCert(siteId),
    onSuccess: () => {
      message.success(tr("site.certProvisioned", "Certificate provisioned"));
      queryClient.invalidateQueries({ queryKey: ["site-cert", siteId] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const renewCertMutation = useMutation({
    mutationFn: () => renewSiteCert(siteId),
    onSuccess: () => {
      message.success(tr("site.certRenewed", "Certificate renewed"));
      queryClient.invalidateQueries({ queryKey: ["site-cert", siteId] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const checkHealthMutation = useMutation({
    mutationFn: () => checkSiteHealth(siteId),
    onSuccess: () => {
      message.success(tr("site.healthChecked", "Health check completed"));
      queryClient.invalidateQueries({ queryKey: ["site-health", siteId] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  return {
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    listQuery,
    detailQuery,
    certQuery,
    healthQuery,
    site,
    siteId,
    matchedSite,
    updateMutation,
    deleteMutation,
    provisionCertMutation,
    renewCertMutation,
    checkHealthMutation,
  };
}
