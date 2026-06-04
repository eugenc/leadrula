import { Link } from "react-router-dom";
import { Building2, Users } from "lucide-react";
import { usePlatformBuyers, usePlatformPublishers } from "@/features/auth/switchHooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/misc";

export function PlatformHomePage() {
  const countFilters = { page: 1, limit: 1 };
  const { data: publishers, isLoading: pubsLoading } = usePlatformPublishers(countFilters);
  const { data: buyers, isLoading: buyersLoading } = usePlatformBuyers(countFilters);

  const pubCount = publishers?.total ?? 0;
  const buyerCount = buyers?.total ?? 0;
  const loading = pubsLoading || buyersLoading;

  return (
    <>
      <PageHeader subtitle="Switch into a publisher or buyer account to manage it, or create new accounts from the lists below." />
      <PageBody>
        {loading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <div className="space-y-6">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="rounded-lg border border-gray-100 bg-surface-card p-5">
                <div className="mb-1 flex items-center gap-2 text-sm font-semibold text-gray-800">
                  <Building2 className="h-4 w-4 text-gray-500" />
                  Publishers
                </div>
                <p className="mb-4 text-3xl font-semibold text-gray-800">{pubCount}</p>
                <Link to="/platform/publishers">
                  <Button variant="secondary" size="sm">
                    Manage publishers
                  </Button>
                </Link>
              </div>
              <div className="rounded-lg border border-gray-100 bg-surface-card p-5">
                <div className="mb-1 flex items-center gap-2 text-sm font-semibold text-gray-800">
                  <Users className="h-4 w-4 text-gray-500" />
                  Buyers
                </div>
                <p className="mb-4 text-3xl font-semibold text-gray-800">{buyerCount}</p>
                <Link to="/platform/buyers">
                  <Button variant="secondary" size="sm">
                    Manage buyers
                  </Button>
                </Link>
              </div>
            </div>
          </div>
        )}
      </PageBody>
    </>
  );
}
