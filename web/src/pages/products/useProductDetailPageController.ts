import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamProduct,
  getIamProduct,
  patchIamProduct,
  buyProduct,
  getProviders,
} from "@/shared/api/iam";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { PRODUCT_STATES } from "@/shared/constants/enums";
import { PRODUCT_TRANSITIONS, allowedTargets } from "@/shared/constants/lifecycles";
import { localizedError } from "@/shared/lib/server-error";

export type ProductFormValues = {
  displayName: string;
  currency: string;
  price: number;
  state: string;
  isRecharge: boolean;
  image: string;
  detail: string;
  description: string;
  tag: string;
  quantity: number;
  sold: number;
  successUrl: string;
  rechargeOptions: string[];
  paymentProviders: string[];
  disableCustomRecharge: boolean;
};

export function useProductDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ProductFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.products.detail(id!),
    queryFn: () => getIamProduct(id!),
    enabled: !!id,
  });

  const paymentProvidersQuery = useQuery({
    queryKey: queryKeys.providers.payment(),
    queryFn: () => getProviders(),
    select: (data) => data.filter((p) => p.category === "Payment"),
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        currency: detailQuery.data.currency,
        price: detailQuery.data.price,
        state: detailQuery.data.state,
        isRecharge: detailQuery.data.isRecharge,
        image: detailQuery.data.image,
        detail: detailQuery.data.detail,
        description: detailQuery.data.description,
        tag: detailQuery.data.tag,
        quantity: detailQuery.data.quantity,
        sold: detailQuery.data.sold ?? 0,
        successUrl: detailQuery.data.successUrl ?? "",
        rechargeOptions: ((detailQuery.data.rechargeOptions ?? []) as unknown[]).map(String),
        paymentProviders: (detailQuery.data.providers ?? []) as string[],
        disableCustomRecharge: detailQuery.data.disableCustomRecharge ?? false,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: ProductFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamProduct(detailQuery.data.id, {
        displayName: values.displayName,
        currency: values.currency,
        price: values.price,
        state: values.state,
        isRecharge: values.isRecharge,
        image: values.image,
        detail: values.detail,
        description: values.description,
        tag: values.tag,
        quantity: values.quantity,
        successUrl: values.successUrl,
        rechargeOptions: (values.rechargeOptions as string[]).map(Number),
        providers: values.paymentProviders,
        disableCustomRecharge: values.disableCustomRecharge,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamProduct(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.all });
      navigate("/products");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const buyMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return buyProduct(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tr("product.buySuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.all });
      void queryClient.invalidateQueries({ queryKey: queryKeys.orders.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "product.buyFailed"));
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
    paymentProvidersQuery,
    updateMutation,
    deleteMutation,
    buyMutation,
    PRODUCT_STATES,
    PRODUCT_TRANSITIONS,
    allowedTargets,
  };
}
