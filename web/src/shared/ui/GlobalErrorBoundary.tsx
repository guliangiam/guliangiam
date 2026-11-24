import { Component, type ErrorInfo, type ReactNode } from "react";
import Result from "antd/es/result";
import Button from "antd/es/button";
import i18n from "@/shared/i18n";

type Props = { children: ReactNode; fallback?: ReactNode };
type State = { hasError: boolean; error: Error | null };

export class GlobalErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("GlobalErrorBoundary caught:", error, info);
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }
      const t = i18n.t.bind(i18n);
      return (
        <div
          style={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            minHeight: "100vh",
          }}
        >
          <Result
            status="error"
            title={t("common:unexpectedError", "An unexpected error occurred.")}
            subTitle={t("common:pageLoadFailed", "Page load failed")}
            extra={[
              <Button
                key="retry"
                type="primary"
                onClick={() => this.setState({ hasError: false, error: null })}
              >
                {t("common:reload", "Retry")}
              </Button>,
              <Button key="home" onClick={() => window.location.assign("/")}>
                {t("common:backToList", "Go to Home")}
              </Button>,
            ]}
          />
        </div>
      );
    }
    return this.props.children;
  }
}
