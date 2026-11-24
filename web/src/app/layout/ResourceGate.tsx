import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import Skeleton from "antd/es/skeleton";

import { ensureResourceNamespace } from "@/shared/i18n";

/**
 * Gate the authenticated shell on the `resource` i18n namespace.
 *
 * Only `common` + `auth` are loaded before first paint (login screen); the
 * `resource` bundle is the largest namespace and is needed exclusively by
 * admin pages, so it is fetched once the shell mounts. Rendering the shell
 * before the bundle arrives would show raw `resource:…` keys.
 */
export function ResourceGate({ children }: { children: React.ReactNode }) {
  const { i18n } = useTranslation();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let alive = true;
    void ensureResourceNamespace(i18n.language).finally(() => {
      if (alive) setReady(true);
    });
    return () => {
      alive = false;
    };
  }, [i18n.language]);

  if (!ready) {
    return (
      <div style={{ padding: 24 }}>
        <Skeleton active paragraph={{ rows: 6 }} />
      </div>
    );
  }

  return <>{children}</>;
}
