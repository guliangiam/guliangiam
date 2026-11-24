import { useEffect, useId } from "react";

type RecaptchaApi = {
  render: (container: string | HTMLElement, options: Record<string, unknown>) => number | string;
  ready?: (callback: () => void) => void;
  execute?: (widgetId: number | string, options?: Record<string, unknown>) => void;
};

type HCaptchaApi = {
  render: (container: string | HTMLElement, options: Record<string, unknown>) => void;
};

type TurnstileApi = {
  render: (container: string | HTMLElement, options: Record<string, unknown>) => void;
};

type GeetestValidateResult = {
  lot_number: string;
  captcha_output: string;
  pass_token: string;
  gen_time: string;
};

type GeetestCaptcha = {
  appendTo: (selector: string) => void;
  onSuccess: (callback: () => void) => void;
  getValidate: () => GeetestValidateResult;
};

type GeetestInit = (
  options: Record<string, unknown>,
  callback: (captchaObj: GeetestCaptcha) => void,
) => void;

type AliyunCaptchaInit = (options: Record<string, unknown>) => void;

type TencentCaptchaResult = {
  ret: number;
  ticket?: string;
  randstr?: string;
};

type TencentCaptchaCtor = new (
  captchaAppId: string,
  callback: (res: TencentCaptchaResult) => void,
  options?: Record<string, unknown>,
) => { show: () => void };

type CaptchaWindow = Window &
  typeof globalThis & {
    grecaptcha?: RecaptchaApi;
    hcaptcha?: HCaptchaApi;
    turnstile?: TurnstileApi;
    initGeetest4?: GeetestInit;
    initAliyunCaptcha?: AliyunCaptchaInit;
    TencentCaptcha?: TencentCaptchaCtor;
    AliyunCaptchaConfig?: {
      region: string;
      prefix: string;
    };
  };

type CaptchaWidgetProps = {
  captchaType: string;
  subType?: string | null;
  siteKey?: string | null;
  clientId2?: string | null;
  captchaConfig?: string | null;
  onChange: (token: string) => void;
};

function loadScriptOnce(src: string) {
  const existing = document.querySelector(`script[src="${src}"]`);
  if (existing) {
    return;
  }

  const tag = document.createElement("script");
  tag.async = false;
  tag.src = src;
  document.body.appendChild(tag);
}

function stopTimer(timer: ReturnType<typeof setInterval> | null) {
  if (timer) {
    clearInterval(timer);
  }
}

export function CaptchaWidget({
  captchaType,
  subType,
  siteKey,
  clientId2,
  captchaConfig,
  onChange,
}: CaptchaWidgetProps) {
  // useId() is stable across renders, so the container id can be read during
  // render without touching a ref.
  const containerId = `captcha-${useId()}`;

  useEffect(() => {
    const globalWindow = window as CaptchaWindow;
    const container = document.getElementById(containerId);
    if (!container) {
      return;
    }

    container.innerHTML = "";
    let timer: ReturnType<typeof setInterval> | null = null;

    // Values are the server's lowercase provider types; `cloudflare`/`aliyun`
    // stay as aliases for rows written before the vocabulary was aligned.
    switch (captchaType) {
      case "recaptcha": {
        timer = setInterval(() => {
          if (!globalWindow.grecaptcha) {
            loadScriptOnce("https://recaptcha.net/recaptcha/api.js");
          }

          if (globalWindow.grecaptcha?.render && siteKey) {
            globalWindow.grecaptcha.render(container, {
              sitekey: siteKey,
              callback: onChange,
            });
            stopTimer(timer);
          }
        }, 300);
        break;
      }
      case "recaptcha_v3": {
        timer = setInterval(() => {
          if (!globalWindow.grecaptcha) {
            loadScriptOnce(`https://recaptcha.net/recaptcha/api.js?render=${siteKey ?? ""}`);
          }

          if (globalWindow.grecaptcha?.render && siteKey) {
            const widgetId = globalWindow.grecaptcha.render(container, {
              sitekey: siteKey,
              badge: "inline",
              size: "invisible",
              callback: onChange,
              "error-callback": () => {},
            });

            globalWindow.grecaptcha.ready?.(() => {
              globalWindow.grecaptcha?.execute?.(widgetId, { action: "submit" });
            });
            stopTimer(timer);
          }
        }, 300);
        break;
      }
      case "hcaptcha": {
        timer = setInterval(() => {
          if (!globalWindow.hcaptcha) {
            loadScriptOnce("https://js.hcaptcha.com/1/api.js");
          }

          if (globalWindow.hcaptcha?.render && siteKey) {
            globalWindow.hcaptcha.render(container, {
              sitekey: siteKey,
              callback: onChange,
            });
            stopTimer(timer);
          }
        }, 300);
        break;
      }
      case "aliyun_captcha":
      case "aliyun": {
        if (captchaConfig) {
          globalWindow.AliyunCaptchaConfig = {
            region: "cn",
            prefix: captchaConfig,
          };
        }

        timer = setInterval(() => {
          if (!globalWindow.initAliyunCaptcha) {
            loadScriptOnce("https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js");
          }

          if (
            globalWindow.initAliyunCaptcha &&
            clientId2 &&
            captchaConfig &&
            captchaConfig !== "***"
          ) {
            globalWindow.initAliyunCaptcha({
              SceneId: clientId2,
              mode: "embed",
              element: `#${containerId}`,
              captchaVerifyCallback: (data: unknown) => {
                onChange(String(data));
              },
              slideStyle: {
                width: 320,
                height: 40,
              },
              language: subType || "cn",
              immediate: true,
            });
            stopTimer(timer);
          }
        }, 300);
        break;
      }
      case "geetest": {
        let attached = false;
        timer = setInterval(() => {
          if (!globalWindow.initGeetest4) {
            loadScriptOnce("https://static.geetest.com/v4/gt4.js");
          }

          if (globalWindow.initGeetest4 && siteKey && !attached) {
            globalWindow.initGeetest4(
              {
                captchaId: siteKey,
                product: "float",
              },
              (captchaObj) => {
                if (!attached) {
                  captchaObj.appendTo(`#${containerId}`);
                  attached = true;
                }

                captchaObj.onSuccess(() => {
                  const result = captchaObj.getValidate();
                  onChange(
                    `lot_number=${result.lot_number}&captcha_output=${result.captcha_output}&pass_token=${result.pass_token}&gen_time=${result.gen_time}&captcha_id=${siteKey}`,
                  );
                });
              },
            );
            stopTimer(timer);
          }
        }, 500);
        break;
      }
      case "tencent_captcha": {
        // Tencent CAPTCHA 2.0 web widget (https://cloud.tencent.com/document/product/1110/36841):
        // `new TencentCaptcha(CaptchaAppId, cb)` pops the challenge; cb receives
        // {ret:0, ticket, randstr} and the pair is forwarded to the server as the
        // URL-encoded "ticket=...&randstr=..." token DescribeCaptchaResult needs.
        let started = false;
        timer = setInterval(() => {
          if (!globalWindow.TencentCaptcha) {
            loadScriptOnce("https://turing.captcha.qcloud.com/TJCaptcha.js");
          }

          if (globalWindow.TencentCaptcha && siteKey && !started) {
            started = true;
            // oxlint-disable-next-line no-new — third-party SDK init requires new for side effects
            new globalWindow.TencentCaptcha(siteKey, (res) => {
              if (res.ret === 0 && res.ticket && res.randstr) {
                onChange(
                  `ticket=${encodeURIComponent(res.ticket)}&randstr=${encodeURIComponent(res.randstr)}`,
                );
              }
            });
            stopTimer(timer);
          }
        }, 500);
        break;
      }
      case "turnstile":
      case "cloudflare": {
        timer = setInterval(() => {
          if (!globalWindow.turnstile) {
            loadScriptOnce("https://challenges.cloudflare.com/turnstile/v0/api.js");
          }

          if (globalWindow.turnstile?.render && siteKey) {
            globalWindow.turnstile.render(container, {
              sitekey: siteKey,
              callback: onChange,
            });
            stopTimer(timer);
          }
        }, 300);
        break;
      }
      default:
        break;
    }

    return () => {
      stopTimer(timer);
    };
  }, [captchaType, clientId2, captchaConfig, containerId, onChange, siteKey, subType]);

  return <div id={containerId} />;
}
