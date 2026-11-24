import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamCoupon, getIamCoupon, patchIamCoupon, validateCoupon } from "@/shared/api/iam";
import { CURRENCY_OPTIONS } from "@/shared/constants/currency";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { COUPON_STATES } from "@/shared/constants/enums";
import { COUPON_TRANSITIONS, allowedTargets } from "@/shared/constants/lifecycles";
import { localizedError } from "@/shared/lib/server-error";

export type CouponFormValues = {
  displayName: string;
  discount: number;
  maxDiscount: number;
  scope: string;
  quantity: number;
  maxUsagePerUser: number;
  currency: string;
  state: string;
  minOrderAmount: number;
  description: string;
};

export function useCouponDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CouponFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.coupons.detail(id!),
    queryFn: () => getIamCoupon(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        discount: detailQuery.data.discount,
        maxDiscount: detailQuery.data.maxDiscount,
        scope: detailQuery.data.scope,
        quantity: detailQuery.data.quantity,
        maxUsagePerUser: detailQuery.data.maxUsagePerUser,
        currency: detailQuery.data.currency,
        state: detailQuery.data.state,
        minOrderAmount: detailQuery.data.minOrderAmount ?? 0,
        description: detailQuery.data.description ?? "",
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: CouponFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamCoupon(detailQuery.data.id, {
        displayName: values.displayName,
        discount: values.discount,
        maxDiscount: values.maxDiscount,
        scope: values.scope,
        quantity: values.quantity,
        maxUsagePerUser: values.maxUsagePerUser,
        currency: values.currency,
        state: values.state,
        minOrderAmount: values.minOrderAmount,
        description: values.description || undefined,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.coupons.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamCoupon(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.coupons.all });
      navigate("/coupons");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const validateMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return validateCoupon({
        code: detailQuery.data.code,
        organizationId: detailQuery.data.organizationId,
      });
    },
    onSuccess: (result) => {
      if (result.isValid) {
        message.success(tr("coupon.valid", { discount: result.discount ?? "N/A" }));
      } else {
        message.warning(
          tr("coupon.invalid", { message: result.message ?? tr("coupon.unknownReason") }),
        );
      }
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "coupon.validateFailed"));
    },
  });

  return {
    id,
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    detailQuery,
    updateMutation,
    deleteMutation,
    validateMutation,
    CURRENCY_OPTIONS,
    COUPON_STATES,
    COUPON_TRANSITIONS,
    allowedTargets,
  };
}
