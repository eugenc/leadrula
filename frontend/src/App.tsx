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

import { IntakeQueuePage } from "@/pages/publisher/IntakeQueuePage";
import { SourcesPage } from "@/pages/publisher/SourcesPage";
import { RoutingPage } from "@/pages/publisher/RoutingPage";
import { ContractsPage } from "@/pages/publisher/ContractsPage";
import { BuyersPage } from "@/pages/publisher/BuyersPage";
import { PublisherBillingPage } from "@/pages/publisher/PublisherBillingPage";

import { CalendarPage } from "@/pages/buyer/CalendarPage";
import { ContractPage } from "@/pages/buyer/ContractPage";
import { BuyerBillingPage } from "@/pages/buyer/BuyerBillingPage";

function RootRedirect() {
  const user = useAuthStore((s) => s.user);
  if (!user) return <Navigate to="/login" replace />;
  return <Navigate to={user.account_type === "publisher" ? "/p" : "/b"} replace />;
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
            {/* Publisher */}
            <Route element={<RequireAccountType type="publisher" />}>
              <Route path="/p" element={<AppShell />}>
                <Route index element={<Dashboard />} />
                <Route path="board" element={<BoardPage />} />
                <Route path="leads" element={<LeadsListPage />} />
                <Route path="intake" element={<IntakeQueuePage />} />
                <Route path="pipelines" element={<PipelinesPage />} />
                <Route path="fields" element={<CustomFieldsPage />} />
                <Route path="reasons" element={<DisqReasonsPage />} />
                <Route path="sources" element={<SourcesPage />} />
                <Route path="routing" element={<RoutingPage />} />
                <Route path="contracts" element={<ContractsPage />} />
                <Route path="buyers" element={<BuyersPage />} />
                <Route path="billing" element={<PublisherBillingPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="api" element={<ApiKeysPage />} />
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
                <Route path="contract" element={<ContractPage />} />
                <Route path="billing" element={<BuyerBillingPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="api" element={<ApiKeysPage />} />
                <Route path="settings" element={<SettingsPage />} />
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
