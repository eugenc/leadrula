package intake

import (
	"context"
	"fmt"
)

const routeLogSelectCols = `
  e.id,
  e.created_at,
  e.route_name,
  e.route_id,
  e.lead_id,
  COALESCE(l.first_name, ''),
  COALESCE(l.last_name, ''),
  l.phone,
  e.status,
  e.trigger_type,
  COALESCE(e.trigger_label, ''),
  e.destination,
  COALESCE(e.branch_position, 0),
  e.owner_account_id,
  e.target_account_id,
  COALESCE(ta.name, ''),
  COALESCE(e.target_pipeline_name, ''),
  COALESCE(e.target_stage_name, ''),
  COALESCE(e.delivery, ''),
  l.raw_payload`

func routeVisibilitySQL(accountType string, argN int) string {
	if accountType == "buyer" {
		return fmt.Sprintf("(e.target_account_id = $%d OR e.owner_account_id = $%d)", argN, argN)
	}
	return fmt.Sprintf("e.owner_account_id = $%d", argN)
}

func routeLogDirection(viewerAccountID, ownerAccountID int64, targetAccountID *int64) string {
	if targetAccountID != nil && *targetAccountID == viewerAccountID && ownerAccountID != viewerAccountID {
		return "inbound"
	}
	return "outbound"
}

func (s *Service) listInboundLogRoutes(ctx context.Context, accountID int64, p ListInboundLogParams) (*InboundLogListResponse, error) {
	limit := p.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	vis := routeVisibilitySQL(p.AccountType, 1)

	args := []any{accountID}
	where := vis
	where, args = appendLogLeadFilter(where, args, p.LeadID, p.Search, "e.lead_id")

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM route_executions e JOIN leads l ON l.id = e.lead_id WHERE `+where,
		args...).Scan(&total); err != nil {
		return nil, err
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx,
		`SELECT`+routeLogSelectCols+`
		 FROM route_executions e
		 JOIN leads l ON l.id = e.lead_id
		 LEFT JOIN accounts ta ON ta.id = e.target_account_id
		 WHERE `+where+`
		 ORDER BY e.created_at DESC
		 LIMIT $`+fmt.Sprint(limitArg)+` OFFSET $`+fmt.Sprint(offsetArg),
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanRouteInboundItems(rows, accountID)
	if err != nil {
		return nil, err
	}
	return &InboundLogListResponse{Items: items, Total: total, Page: page, Limit: limit}, nil
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanRouteInboundItems(rows rowScanner, viewerAccountID int64) ([]InboundLogItem, error) {
	var items []InboundLogItem
	for rows.Next() {
		var it InboundLogItem
		var routeID *int64
		var leadID int64
		var ownerID int64
		var targetID *int64
		var targetPipelineName, targetStageName string
		var raw []byte
		if err := rows.Scan(
			&it.ID, &it.CreatedAt, &it.RouteName, &routeID, &leadID,
			&it.FirstName, &it.LastName, &it.Phone, &it.Status,
			&it.TriggerType, &it.TriggerLabel, &it.Destination, &it.BranchPosition,
			&ownerID, &targetID, &it.TargetAccountName,
			&targetPipelineName, &targetStageName, &it.Delivery,
			&raw,
		); err != nil {
			return nil, err
		}
		if targetPipelineName != "" {
			it.TargetPipelineName = &targetPipelineName
		}
		if targetStageName != "" {
			it.TargetStageName = &targetStageName
		}
		it.Kind = "route"
		it.Direction = routeLogDirection(viewerAccountID, ownerID, targetID)
		it.RouteID = routeID
		it.Origin = it.RouteName
		it.OriginSlug = it.RouteName
		it.LeadID = &leadID
		it.LeadLabel = fmt.Sprintf("%s %s", it.FirstName, it.LastName)
		if len(raw) > 0 {
			it.RawPayload = raw
		}
		items = append(items, it)
	}
	if items == nil {
		items = []InboundLogItem{}
	}
	return items, rows.Err()
}

func buildRouteLogUnionSQL(accountType string, leadFilter string) string {
	vis := routeVisibilitySQL(accountType, 1)
	return `
	   SELECT
	     'route'::text AS kind,
	     CASE
	       WHEN e.target_account_id = $1 AND e.owner_account_id <> $1 THEN 'inbound'::text
	       ELSE 'outbound'::text
	     END AS direction,
	     e.id,
	     e.created_at,
	     e.route_name AS origin,
	     e.route_name AS origin_slug,
	     TRIM(CONCAT(l.first_name, ' ', l.last_name)) AS lead_label,
	     e.lead_id,
	     e.status::text AS status,
	     l.first_name,
	     l.last_name,
	     l.phone,
	     NULL::text AS source,
	     l.raw_payload,
	     0::bigint AS webhook_id,
	     e.error_message,
	     ''::text AS provider_slug,
	     ''::text AS connection_name,
	     0::int AS attempts,
	     e.route_id,
	     e.route_name,
	     e.trigger_type,
	     COALESCE(e.trigger_label, '') AS trigger_label,
	     COALESCE(ta.name, '') AS target_account_name,
	     e.destination,
	     COALESCE(e.branch_position, 0) AS branch_position,
	     COALESCE(e.target_pipeline_name, '') AS target_pipeline_name,
	     COALESCE(e.target_stage_name, '') AS target_stage_name,
	     COALESCE(e.delivery, '') AS delivery
	   FROM route_executions e
	   JOIN leads l ON l.id = e.lead_id
	   LEFT JOIN accounts ta ON ta.id = e.target_account_id
	   WHERE ` + vis + leadFilter
}
