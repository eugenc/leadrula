package leads

import (
	"context"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/internal/routing"
)

// RouteExecutionMeta describes how a route was triggered for audit logging.
type RouteExecutionMeta struct {
	TriggerType  string
	TriggerLabel string
	ReviewerID   int64
}

// RecordRouteExecutionParams is the insert payload for route_executions.
type RecordRouteExecutionParams struct {
	RouteID             *int64
	RouteName           string
	LeadID              int64
	OwnerAccountID      int64
	TargetAccountID     *int64
	Destination         string
	TriggerType         string
	TriggerLabel        string
	BranchPosition      int
	ReviewerID          int64
	TargetPipelineID    *int64
	TargetStageID       *int64
	TargetPipelineName  *string
	TargetStageName     *string
	Delivery            string
}

// RecordRouteExecution inserts a successful route execution audit row.
func RecordRouteExecution(ctx context.Context, q database.Querier, p RecordRouteExecutionParams) error {
	var routeID *int64
	if p.RouteID != nil && *p.RouteID > 0 {
		routeID = p.RouteID
	}
	var reviewedBy *int64
	if p.ReviewerID > 0 {
		reviewedBy = &p.ReviewerID
	}
	var branchPos *int
	if p.BranchPosition > 0 {
		branchPos = &p.BranchPosition
	}
	_, err := q.Exec(ctx,
		`INSERT INTO route_executions(
		   route_id, route_name, lead_id, owner_account_id, target_account_id,
		   destination, trigger_type, trigger_label, branch_position, reviewed_by,
		   target_pipeline_id, target_stage_id, target_pipeline_name, target_stage_name, delivery)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		routeID, p.RouteName, p.LeadID, p.OwnerAccountID, p.TargetAccountID,
		p.Destination, p.TriggerType, nullIfEmpty(p.TriggerLabel), branchPos, reviewedBy,
		p.TargetPipelineID, p.TargetStageID, p.TargetPipelineName, p.TargetStageName, nullIfEmpty(p.Delivery))
	return err
}

func recordRouteFromRoute(ctx context.Context, q database.Querier, route *routing.Route, leadID int64, meta RouteExecutionMeta, targetAccountID *int64) error {
	routeID := route.ID
	label := meta.TriggerLabel
	if label == "" {
		label = defaultTriggerLabel(route)
	}
	p := RecordRouteExecutionParams{
		RouteID:         &routeID,
		RouteName:       route.Name,
		LeadID:          leadID,
		OwnerAccountID:  route.OwnerAccountID(),
		TargetAccountID: targetAccountID,
		Destination:     route.Destination,
		TriggerType:     meta.TriggerType,
		TriggerLabel:    label,
		BranchPosition:  route.MatchedBranchPosition,
		ReviewerID:      meta.ReviewerID,
	}
	if route.Destination == "pipeline" {
		p.TargetPipelineID = route.TargetPipelineID
		p.TargetStageID = route.TargetStageID
		p.Delivery = route.Delivery
		if route.TargetPipelineID != nil && route.TargetStageID != nil {
			pipelineName, stageName := resolveRouteTargetNames(ctx, q, *route.TargetPipelineID, *route.TargetStageID)
			p.TargetPipelineName = pipelineName
			p.TargetStageName = stageName
		} else {
			p.TargetPipelineName = route.TargetPipelineName
			p.TargetStageName = route.TargetStageName
		}
	}
	return RecordRouteExecution(ctx, q, p)
}

func resolveRouteTargetNames(ctx context.Context, q database.Querier, pipelineID, stageID int64) (*string, *string) {
	var pipelineName, stageName string
	err := q.QueryRow(ctx,
		`SELECT p.name, ps.name FROM pipeline_stages ps
		 JOIN pipelines p ON p.id = ps.pipeline_id
		 WHERE ps.id = $1 AND ps.pipeline_id = $2`,
		stageID, pipelineID).Scan(&pipelineName, &stageName)
	if err != nil {
		return nil, nil
	}
	return &pipelineName, &stageName
}

func defaultTriggerLabel(route *routing.Route) string {
	if route.OriginStageName != nil && *route.OriginStageName != "" {
		return *route.OriginStageName
	}
	if route.SourceName != nil && *route.SourceName != "" {
		return *route.SourceName
	}
	if route.OriginWebhookName != nil && *route.OriginWebhookName != "" {
		return *route.OriginWebhookName
	}
	if route.OriginConnectionName != nil && *route.OriginConnectionName != "" {
		return *route.OriginConnectionName
	}
	return ""
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
