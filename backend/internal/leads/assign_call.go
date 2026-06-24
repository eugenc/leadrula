package leads

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/contracts"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/notifications"
)

// AssignCallLeadAfterBill places a call lead with the winning buyer AFTER the
// caller (the calls package) has already debited the buyer at the call price.
// It mirrors the placement side of ApplyContractDistribution (field maps,
// pipeline/inbox placement, integration enqueue, notification) but never bills:
// call billing is duration-based and lives in the calls package.
//
// participationID is the winning participation; the lead lands in the buyer
// inbox ("leads") or pipeline ("leads_pipeline") per that participation's delivery.
func AssignCallLeadAfterBill(ctx context.Context, q database.Querier, deps RouteApplyDeps, contractID, participationID, leadID int64) ([]notifications.EmailJob, error) {
	target, err := contracts.GetTargetByParticipation(ctx, q, participationID)
	if err != nil {
		return nil, err
	}
	lead, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	if err := LoadCustomValues(ctx, q, lead); err != nil {
		return nil, err
	}
	// Same field mapping as Data leads: apply the contract/participation field map.
	contractMaps, err := contracts.ContractFieldMapForRoute(ctx, q, contractID, target.ParticipationID)
	if err != nil {
		return nil, err
	}
	if len(contractMaps) > 0 {
		if err := ApplyRouteFieldMap(ctx, q, deps.Repo, lead, contractMaps); err != nil {
			return nil, err
		}
	}

	tcID := target.ID
	delivery := target.Delivery
	if delivery == "" {
		delivery = "leads"
	}

	if delivery == "leads" || delivery == "webhook" {
		if err := deps.Repo.TransferOwner(ctx, q, leadID, target.BuyerID, &tcID); err != nil {
			return nil, err
		}
		if err := deps.Repo.SetStatusWithLog(ctx, q, leadID, ActorSystem("Call"), "review"); err != nil {
			return nil, err
		}
		return nil, enqueueParticipationIntegration(ctx, deps, target, lead)
	}

	destStage, err := resolveBuyerDestStage(ctx, q, target.BuyerStageID, target.BuyerPipelineID)
	if err != nil {
		return nil, err
	}
	if err := deps.Repo.PlaceInPipeline(ctx, q, leadID, target.BuyerID, target.BuyerPipelineID, destStage, &tcID); err != nil {
		return nil, err
	}
	if err := deps.Repo.LogPipelinePlacement(ctx, q, leadID, ActorSystem("Call"), target.BuyerPipelineID, destStage); err != nil {
		return nil, err
	}
	if err := contracts.ClearPublisherTracking(ctx, q, leadID); err != nil {
		return nil, err
	}
	if err := deps.Repo.SetStatusWithLog(ctx, q, leadID, ActorSystem("Call"), "distributed"); err != nil {
		return nil, err
	}
	updated, err := deps.Repo.GetByID(ctx, q, leadID)
	if err != nil {
		return nil, err
	}
	emails, err := deps.Notif.Deliver(ctx, q, notifications.DeliverParams{
		AccountID: target.BuyerID,
		UserIDs:   notifications.AssigneeIDs(updated.AssignedUserID),
		EventType: "new_lead",
		Payload:   map[string]any{"lead_id": leadID},
	})
	if err != nil {
		return nil, err
	}
	if err := enqueueParticipationIntegration(ctx, deps, target, updated); err != nil {
		return nil, err
	}
	return emails, nil
}
