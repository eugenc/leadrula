import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { PageBody } from "@/components/layout/PageBody";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { apiError, errorMessage, get } from "@/lib/api";
import { useAttachContractInvite } from "@/features/admin/hooks";
import { BuyerParticipationAcceptDrawer } from "@/features/admin/BuyerParticipationAcceptDrawer";
import type { ContractParticipation } from "@/types";

export function ContractInvitePage() {
  const { token = "" } = useParams();
  const navigate = useNavigate();
  const { mutateAsync: attachInvite } = useAttachContractInvite();
  const started = useRef(false);
  const [participation, setParticipation] = useState<ContractParticipation | null>(null);
  const [loading, setLoading] = useState(!!token);
  const [error, setError] = useState(token ? "" : "Invalid invite link");
  const [alreadyInvited, setAlreadyInvited] = useState(false);

  useEffect(() => {
    if (!token || started.current) return;
    started.current = true;

    void (async () => {
      try {
        const part = await attachInvite(token);
        const participations = await get<ContractParticipation[]>("/buyer/participations");
        const full = participations.find((p) => p.id === part.id) ?? part;
        setParticipation(full);
      } catch (e) {
        const err = apiError(e);
        if (err.status === 409 || err.code === "conflict") {
          setAlreadyInvited(true);
          setError("You're already invited to this contract.");
        } else if (err.status === 404 || err.code === "not_found") {
          setError("Invite not found or no longer valid.");
        } else {
          setError(errorMessage(e));
        }
      } finally {
        setLoading(false);
      }
    })();
  }, [token, attachInvite]);

  if (loading) {
    return (
      <PageBody>
        <div className="flex justify-center py-12">
          <Spinner className="h-6 w-6" />
        </div>
      </PageBody>
    );
  }

  if (error) {
    return (
      <PageBody>
        <EmptyState
          title={alreadyInvited ? "Already invited" : "Could not open invite"}
          subtitle={error}
        />
        {alreadyInvited && (
          <p className="mt-4 text-center text-sm">
            <Link to="/b/contract" className="text-jade-600 hover:underline">
              View your contracts
            </Link>
          </p>
        )}
      </PageBody>
    );
  }

  return (
    <>
      <PageBody>
        <p className="text-sm text-gray-500">Opening contract invitation…</p>
      </PageBody>
      <BuyerParticipationAcceptDrawer
        participation={participation}
        onClose={() => navigate("/b/contract")}
      />
    </>
  );
}
