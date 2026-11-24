import { Result, Skeleton } from "antd";
import { ConsentPageCard } from "./ConsentPageCard";
import { useConsentPageController } from "./useConsentPageController";

export default function ConsentPage() {
  const controller = useConsentPageController();

  if (controller.status === "loading") {
    return (
      <div
        className="login-content"
        style={{ margin: "auto", minHeight: "60vh", display: "grid", placeItems: "center" }}
      >
        <Skeleton active paragraph={{ rows: 4 }} />
      </div>
    );
  }

  if (controller.status !== "ready") {
    return (
      <div className="login-content" style={{ margin: "auto" }}>
        <Result status="error" title={controller.errorTitle} />
      </div>
    );
  }

  return <ConsentPageCard {...controller} />;
}
