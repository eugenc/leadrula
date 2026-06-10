import { Outlet } from "react-router-dom";
import { PageBody } from "@/components/layout/PageBody";

export function IntegrationsLayout() {
  return (
    <PageBody>
      <Outlet />
    </PageBody>
  );
}
