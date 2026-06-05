import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "@/lib/queryClient";
import { useAuthStore } from "@/store/authStore";
import { RequireAuth, RequireAccountType } from "@/components/auth/guards";
import { AppShell } from "@/components/layout/AppShell";
import { Toaster } from "@/components/ui/toaster";

import { LoginPage } from "@/features/auth/LoginPage";
import { InviteAcceptPage } from "@/features/auth/InviteAcceptPage";
import { ResetPasswordPage } from "@/features/auth/ResetPasswordPage";
import { ForgotPasswordPage } from "@/features/auth/ForgotPasswordPage";
import { Dashboard } from "@/pages/Dashboard";
import { BoardPage } from "@/pages/BoardPage";
import { LeadsListPage } from "@/pages/LeadsListPage";
import { PipelinesPage } from "@/pages/PipelinesPage";
import { CustomFieldsPage } from "@/pages/CustomFieldsPage";
import { DisqReasonsPage } from "@/pages/DisqReasonsPage";
import { UsersPage } from "@/pages/UsersPage";
import { SettingsPage } from "@/pages/SettingsPage";
import { ApiKeysPage } from "@/pages/buyer/ApiKeysPage";

import { IntakeLogLayout } from "@/pages/publisher/IntakeLogLayout";
import { IntakeLogTab } from "@/pages/publisher/IntakeLogTab";
import { IntakeReviewTab } from "@/pages/publisher/IntakeReviewTab";
import { SourcesPage } from "@/pages/publisher/SourcesPage";
import { WebhooksPage } from "@/pages/WebhooksPage";
import { RoutingPage } from "@/pages/publisher/RoutingPage";
import { ContractsPage } from "@/pages/publisher/ContractsPage";
import { BuyersPage } from "@/pages/publisher/BuyersPage";
import { PublisherCollaborationLayout } from "@/pages/publisher/CollaborationLayout";
import { PublisherCollaborationAccessTab } from "@/pages/publisher/CollaborationAccessTab";
import { PublisherCollaborationActivityTab } from "@/pages/publisher/CollaborationActivityTab";
import { PublisherBillingPage } from "@/pages/publisher/PublisherBillingPage";

import { CalendarPage } from "@/pages/buyer/CalendarPage";
import { ContractPage } from "@/pages/buyer/ContractPage";
import { BuyerBillingPage } from "@/pages/buyer/BuyerBillingPage";
import { CollaborationLayout } from "@/pages/buyer/CollaborationLayout";
import { CollaborationAccessTab } from "@/pages/buyer/CollaborationAccessTab";
import { CollaborationActivityTab } from "@/pages/buyer/CollaborationActivityTab";
import { PublishersPage } from "@/pages/buyer/PublishersPage";
import { RoutesPage } from "@/pages/buyer/RoutesPage";
import { LogsPage } from "@/pages/buyer/LogsPage";
import { PlatformShell } from "@/components/layout/PlatformShell";
import { PlatformHomePage } from "@/pages/platform/PlatformHomePage";
import { PlatformPublishersPage } from "@/pages/platform/PlatformPublishersPage";
import { PlatformBuyersPage } from "@/pages/platform/PlatformBuyersPage";
import { IntegrationsLayout } from "@/pages/IntegrationsLayout";
import { IntegrationsConnectionsTab } from "@/pages/IntegrationsConnectionsTab";
import { IntegrationsDeliveriesTab } from "@/pages/IntegrationsDeliveriesTab";
import { homePath } from "@/lib/homePath";

function RootRedirect() {
  const user = useAuthStore((s) => s.user);
  if (!user) return <Navigate to="/login" replace />;
  return <Navigate to={homePath(user.account_type)} replace />;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/invite/accept" element={<InviteAcceptPage />} />
          <Route path="/reset" element={<ResetPasswordPage />} />
          <Route path="/forgot-password" element={<ForgotPasswordPage />} />
          <Route path="/" element={<RootRedirect />} />

          <Route element={<RequireAuth />}>
            {/* Platform operator */}
            <Route element={<RequireAccountType type="platform" />}>
              <Route path="/platform" element={<PlatformShell />}>
                <Route index element={<PlatformHomePage />} />
                <Route path="publishers" element={<PlatformPublishersPage />} />
                <Route path="buyers" element={<PlatformBuyersPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="settings" element={<SettingsPage />} />
              </Route>
            </Route>

            {/* Publisher */}
            <Route element={<RequireAccountType type="publisher" />}>
              <Route path="/p" element={<AppShell />}>
                <Route index element={<Dashboard />} />
                <Route path="board" element={<BoardPage />} />
                <Route path="leads" element={<LeadsListPage />} />
                <Route path="log" element={<IntakeLogLayout />}>
                  <Route index element={<IntakeLogTab />} />
                  <Route path="review" element={<IntakeReviewTab />} />
                </Route>
                <Route path="intake" element={<Navigate to="/p/log" replace />} />
                <Route path="pipelines" element={<PipelinesPage />} />
                <Route path="fields" element={<CustomFieldsPage />} />
                <Route path="reasons" element={<DisqReasonsPage />} />
                <Route path="sources" element={<SourcesPage />} />
                <Route path="webhooks" element={<WebhooksPage />} />
                <Route path="routing" element={<RoutingPage />} />
                <Route path="contracts" element={<ContractsPage />} />
                <Route path="buyers" element={<BuyersPage />} />
                <Route path="collaboration" element={<PublisherCollaborationLayout />}>
                  <Route index element={<PublisherCollaborationAccessTab />} />
                  <Route path="activity" element={<PublisherCollaborationActivityTab />} />
                </Route>
                <Route path="billing" element={<PublisherBillingPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="api" element={<ApiKeysPage />} />
                <Route path="webhooks" element={<WebhooksPage />} />
                <Route path="integrations" element={<IntegrationsLayout />}>
                  <Route index element={<IntegrationsConnectionsTab />} />
                  <Route path="deliveries" element={<IntegrationsDeliveriesTab />} />
                </Route>
                <Route path="settings" element={<SettingsPage />} />
              </Route>
            </Route>

            {/* Buyer */}
            <Route element={<RequireAccountType type="buyer" />}>
              <Route path="/b" element={<AppShell />}>
                <Route index element={<Dashboard />} />
                <Route path="board" element={<BoardPage />} />
                <Route path="leads" element={<LeadsListPage />} />
                <Route path="calendar" element={<CalendarPage />} />
                <Route path="pipelines" element={<PipelinesPage />} />
                <Route path="fields" element={<CustomFieldsPage />} />
                <Route path="reasons" element={<DisqReasonsPage />} />
                <Route path="publishers" element={<PublishersPage />} />
                <Route path="contract" element={<ContractPage />} />
                <Route path="routes" element={<RoutesPage />} />
                <Route path="collaboration" element={<CollaborationLayout />}>
                  <Route index element={<CollaborationAccessTab />} />
                  <Route path="activity" element={<CollaborationActivityTab />} />
                </Route>
                <Route path="logs" element={<LogsPage />} />
                <Route path="billing" element={<BuyerBillingPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="api" element={<ApiKeysPage />} />
                <Route path="webhooks" element={<WebhooksPage />} />
                <Route path="integrations" element={<IntegrationsLayout />}>
                  <Route index element={<IntegrationsConnectionsTab />} />
                  <Route path="deliveries" element={<IntegrationsDeliveriesTab />} />
                </Route>
                <Route path="settings" element={<SettingsPage />} />
                <Route path="settings/collaboration" element={<Navigate to="/b/collaboration" replace />} />
              </Route>
            </Route>
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
        <Toaster />
      </BrowserRouter>
    </QueryClientProvider>
  );
}
