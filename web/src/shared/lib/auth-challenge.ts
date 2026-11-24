import type { ResponseEnvelope } from "@/shared/api/http";
import type { MfaPropsItem } from "@/shared/types/domain";
import { t } from "i18next";

export type ChallengePendingState<TPayload> = {
  loginValues: TPayload;
  mfaProps: MfaPropsItem[];
};

function toChallengePendingState<TPayload>(values: {
  payload: TPayload;
  response: ResponseEnvelope;
}): ChallengePendingState<TPayload> {
  return {
    loginValues: values.payload,
    mfaProps: Array.isArray(values.response.data2) ? (values.response.data2 as MfaPropsItem[]) : [],
  };
}

export async function submitCasChallenge<TPayload>(values: {
  payload: TPayload;
  submit: (payload: TPayload) => Promise<ResponseEnvelope>;
  onPendingMfa: (pending: ChallengePendingState<TPayload>) => void | Promise<void>;
  showInfo: (message: string) => void;
  onComplete: (response: ResponseEnvelope, payload: TPayload) => Promise<void>;
  errorMessage?: string;
}) {
  const response = await values.submit(values.payload);

  if (response.status !== "ok") {
    throw new Error(response.msg || values.errorMessage || t("auth:loginFailed"));
  }

  if (response.data === "NextMfa") {
    await values.onPendingMfa(
      toChallengePendingState({
        payload: values.payload,
        response,
      }),
    );
    values.showInfo(t("auth:mfaSecondVerify"));
    return;
  }

  await values.onComplete(response, values.payload);
}
