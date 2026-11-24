import { renderCrudListView } from "@/shared/ui/CrudListPage";
import type { ApplicationListParams } from "@/shared/api/iam/application";
import type { ApplicationRow } from "./application-list-helpers";
import {
  useApplicationsPageController,
  type ApplicationDeleteKey,
} from "./useApplicationsPageController";

export default function ApplicationsPage() {
  const { crud, ...config } = useApplicationsPageController();

  return renderCrudListView<ApplicationRow, ApplicationListParams, ApplicationDeleteKey>(
    config,
    crud,
  );
}
