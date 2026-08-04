package pipelines

import (
	"context"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/internal/integrations/providers"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

// CRMImportHelper merges CRM pipeline/stage maps and reads connection config during import.
type CRMImportHelper interface {
	MergeCRMPipelineStageMap(ctx context.Context, accountID, connectionID int64, entries []providers.GHLPipelineStageMapEntry) error
	ConnectionStageMap(ctx context.Context, accountID, connectionID int64) ([]providers.GHLPipelineStageMapEntry, error)
}

type ImportCRMStageInput struct {
	ExternalID   string `json:"external_id"`
	Name         string `json:"name"`
	Position     int    `json:"position"`
	IsWon        bool   `json:"is_won,omitempty"`
	IsClosedLost bool   `json:"is_closed_lost,omitempty"`
	IsClosed     bool   `json:"is_closed,omitempty"`
}

type ImportCRMPipelineInput struct {
	ExternalID string                `json:"external_id"`
	Name       string                `json:"name"`
	Stages     []ImportCRMStageInput `json:"stages"`
}

type ImportFromCRMInput struct {
	ConnectionID    int64                    `json:"connection_id"`
	ProviderSlug    string                   `json:"provider_slug"`
	SetupCRMMapping bool                     `json:"setup_crm_mapping"`
	SetupGHLMapping bool                     `json:"setup_ghl_mapping"`
	Pipelines       []ImportCRMPipelineInput `json:"pipelines"`
}

type ImportedPipelineSummary struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	StageCount int    `json:"stage_count"`
}

type SyncedPipelineSummary struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	StagesAdded     int    `json:"stages_added"`
	StagesRenamed   int    `json:"stages_renamed"`
	StagesReordered bool   `json:"stages_reordered"`
}

type RenamedPipelineSummary struct {
	OriginalName string `json:"original_name"`
	FinalName    string `json:"final_name"`
	ID           int64  `json:"id"`
	StageCount   int    `json:"stage_count"`
}

type SkippedPipelineSummary struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ImportFromCRMResult struct {
	Created []ImportedPipelineSummary `json:"created"`
	Synced  []SyncedPipelineSummary   `json:"synced"`
	Renamed []RenamedPipelineSummary  `json:"renamed"`
	Skipped []SkippedPipelineSummary  `json:"skipped"`
}

func (s *Service) ImportFromCRM(ctx context.Context, p *auth.Principal, in ImportFromCRMInput, crmHelper CRMImportHelper) (*ImportFromCRMResult, error) {
	if in.ConnectionID == 0 {
		return nil, httpx.Validation("connection_id required")
	}
	if !providers.CRMPipelineImportSupported(in.ProviderSlug) {
		return nil, httpx.Validation(in.ProviderSlug + " does not support pipeline import")
	}
	if len(in.Pipelines) == 0 {
		return nil, httpx.Validation("no pipelines to import")
	}

	existingNames, err := s.pipelineNames(ctx, p.AccountID)
	if err != nil {
		return nil, err
	}

	var stageMap []providers.GHLPipelineStageMapEntry
	if crmHelper != nil {
		stageMap, _ = crmHelper.ConnectionStageMap(ctx, p.AccountID, in.ConnectionID)
	}

	result := &ImportFromCRMResult{}
	var mapEntries []providers.GHLPipelineStageMapEntry
	providerLabel := providers.ProviderDisplayName(in.ProviderSlug)
	setupMapping := in.SetupCRMMapping || in.SetupGHLMapping

	for _, pipeIn := range in.Pipelines {
		name := strings.TrimSpace(pipeIn.Name)
		if name == "" {
			result.Skipped = append(result.Skipped, SkippedPipelineSummary{Name: pipeIn.ExternalID, Reason: "empty pipeline name"})
			continue
		}
		if len(pipeIn.Stages) == 0 {
			result.Skipped = append(result.Skipped, SkippedPipelineSummary{Name: name, Reason: "no stages"})
			continue
		}

		if pl, found := s.pipelineByExactName(ctx, p, name); found {
			synced, stages, err := s.syncPipelineStages(ctx, p, pl, pipeIn, stageMap)
			if err != nil {
				return nil, err
			}
			result.Synced = append(result.Synced, synced)
			if setupMapping {
				mapEntries = append(mapEntries, buildMapEntries(pl.ID, pipeIn, stages)...)
			}
			continue
		}

		finalName, renamed := resolveImportPipelineName(name, providerLabel, existingNames)
		pl, stages, err := s.importPipelineWithStages(ctx, p, finalName, pipeIn.Stages)
		if err != nil {
			return nil, err
		}
		existingNames[strings.ToLower(finalName)] = true

		if renamed {
			result.Renamed = append(result.Renamed, RenamedPipelineSummary{
				OriginalName: name,
				FinalName:    finalName,
				ID:           pl.ID,
				StageCount:   len(stages),
			})
		} else {
			result.Created = append(result.Created, ImportedPipelineSummary{
				ID:         pl.ID,
				Name:       finalName,
				StageCount: len(stages),
			})
		}

		if setupMapping {
			mapEntries = append(mapEntries, buildMapEntries(pl.ID, pipeIn, stages)...)
		}
	}

	if len(mapEntries) > 0 && crmHelper != nil {
		if err := crmHelper.MergeCRMPipelineStageMap(ctx, p.AccountID, in.ConnectionID, mapEntries); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func buildMapEntries(pipelineID int64, pipeIn ImportCRMPipelineInput, stages []Stage) []providers.GHLPipelineStageMapEntry {
	out := make([]providers.GHLPipelineStageMapEntry, 0, len(stages))
	for i, st := range stages {
		extStageID := ""
		if i < len(pipeIn.Stages) {
			extStageID = pipeIn.Stages[i].ExternalID
		}
		entry := providers.GHLPipelineStageMapEntry{
			LeadrulaPipelineID: pipelineID,
			LeadrulaStageID:    st.ID,
			CRMPipelineID:      pipeIn.ExternalID,
			CRMStageID:         extStageID,
		}
		if pipeIn.ExternalID != "" {
			entry.GHLPipelineID = pipeIn.ExternalID
		}
		if extStageID != "" {
			entry.GHLPipelineStageID = extStageID
		}
		out = append(out, entry)
	}
	return out
}

func (s *Service) pipelineByExactName(ctx context.Context, p *auth.Principal, name string) (*Pipeline, bool) {
	pl := &Pipeline{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, public_id, account_id, name, position, created_at
		 FROM pipelines WHERE account_id = $1 AND lower(name) = lower($2) LIMIT 1`,
		p.AccountID, name).Scan(&pl.ID, &pl.PublicID, &pl.AccountID, &pl.Name, &pl.Position, &pl.CreatedAt)
	if err != nil {
		return nil, false
	}
	return pl, true
}

func (s *Service) syncPipelineStages(ctx context.Context, p *auth.Principal, pl *Pipeline, pipeIn ImportCRMPipelineInput, stageMap []providers.GHLPipelineStageMapEntry) (SyncedPipelineSummary, []Stage, error) {
	summary := SyncedPipelineSummary{ID: pl.ID, Name: pl.Name}
	existing, err := s.ListStages(ctx, p, pl.ID)
	if err != nil {
		return summary, nil, err
	}

	crmToLR := providers.StageMapCRMStageToLeadrula(providers.NormalizePipelineStageMapEntries(stageMap), pl.ID, pipeIn.ExternalID)
	byID := map[int64]*Stage{}
	byName := map[string]*Stage{}
	for i := range existing {
		st := existing[i]
		byID[st.ID] = &st
		byName[strings.ToLower(st.Name)] = &st
	}

	matched := map[int64]bool{}
	type staged struct {
		stage   Stage
		crmIdx  int
		created bool
	}
	var ordered []staged

	for i, stIn := range pipeIn.Stages {
		stName := strings.TrimSpace(stIn.Name)
		if stName == "" {
			continue
		}
		var target *Stage
		if lrID, ok := crmToLR[stIn.ExternalID]; ok {
			target = byID[lrID]
		}
		if target == nil {
			target = byName[strings.ToLower(stName)]
		}
		if target != nil {
			matched[target.ID] = true
			crmStage := providers.CRMStage{
				ExternalID: stIn.ExternalID, Name: stName, Position: i,
				IsWon: stIn.IsWon, IsClosedLost: stIn.IsClosedLost, IsClosed: stIn.IsClosed,
			}
			stageType := providers.InferCRMStageType(crmStage, i == len(pipeIn.Stages)-1)
			if err := ValidateStageType(stageType); err != nil {
				stageType = StageTypeStandard
			}
			if target.Name != stName {
				updated, err := s.UpdateStage(ctx, p, target.ID, &stName, nil, &stageType)
				if err != nil {
					return summary, nil, err
				}
				target = updated
				summary.StagesRenamed++
			} else if target.StageType != stageType {
				updated, err := s.UpdateStage(ctx, p, target.ID, nil, nil, &stageType)
				if err != nil {
					return summary, nil, err
				}
				target = updated
			}
			ordered = append(ordered, staged{stage: *target, crmIdx: i})
			continue
		}

		crmStage := providers.CRMStage{
			ExternalID: stIn.ExternalID, Name: stName, Position: i,
			IsWon: stIn.IsWon, IsClosedLost: stIn.IsClosedLost, IsClosed: stIn.IsClosed,
		}
		stageType := providers.InferCRMStageType(crmStage, i == len(pipeIn.Stages)-1)
		if err := ValidateStageType(stageType); err != nil {
			stageType = StageTypeStandard
		}
		created, err := s.CreateStage(ctx, p, pl.ID, stName, "gray", stageType)
		if err != nil {
			return summary, nil, err
		}
		summary.StagesAdded++
		ordered = append(ordered, staged{stage: *created, crmIdx: i, created: true})
		matched[created.ID] = true
	}

	for i := range existing {
		st := existing[i]
		if matched[st.ID] {
			continue
		}
		ordered = append(ordered, staged{stage: st, crmIdx: -1})
	}

	ids := make([]int64, 0, len(ordered))
	finalStages := make([]Stage, 0, len(ordered))
	for i, item := range ordered {
		if item.stage.Position != i {
			summary.StagesReordered = true
		}
		ids = append(ids, item.stage.ID)
		st := item.stage
		st.Position = i
		finalStages = append(finalStages, st)
	}
	if summary.StagesReordered || summary.StagesAdded > 0 {
		if err := s.Reorder(ctx, p, pl.ID, ids); err != nil {
			return summary, nil, err
		}
	}

	return summary, finalStages, nil
}

func (s *Service) pipelineNames(ctx context.Context, accountID int64) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT lower(name) FROM pipelines WHERE account_id = $1`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names[n] = true
	}
	return names, rows.Err()
}

func resolveImportPipelineName(name, providerLabel string, existing map[string]bool) (final string, renamed bool) {
	candidate := name
	if existing[strings.ToLower(candidate)] {
		candidate = name + " (" + providerLabel + ")"
		renamed = true
	}
	if !existing[strings.ToLower(candidate)] {
		return candidate, renamed
	}
	for i := 2; i < 100; i++ {
		next := candidate + " (" + strconv.Itoa(i) + ")"
		if !existing[strings.ToLower(next)] {
			return next, true
		}
	}
	return candidate + " (import)", true
}

func (s *Service) importPipelineWithStages(ctx context.Context, p *auth.Principal, name string, stagesIn []ImportCRMStageInput) (*Pipeline, []Stage, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	pubID, scoped := p.OversightPublisherID()
	var collabPub any
	if scoped {
		collabPub = pubID
	}

	pl := &Pipeline{}
	err = tx.QueryRow(ctx,
		`INSERT INTO pipelines(account_id, name, position, collaboration_publisher_id)
		 VALUES ($1, $2, COALESCE((SELECT MAX(position)+1 FROM pipelines WHERE account_id=$1), 0), $3)
		 RETURNING id, public_id, account_id, name, position, created_at`,
		p.AccountID, name, collabPub).Scan(&pl.ID, &pl.PublicID, &pl.AccountID, &pl.Name, &pl.Position, &pl.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	stages := make([]Stage, 0, len(stagesIn))
	for i, stIn := range stagesIn {
		stName := strings.TrimSpace(stIn.Name)
		if stName == "" {
			continue
		}
		crmStage := providers.CRMStage{
			ExternalID: stIn.ExternalID, Name: stName, Position: stIn.Position,
			IsWon: stIn.IsWon, IsClosedLost: stIn.IsClosedLost, IsClosed: stIn.IsClosed,
		}
		if crmStage.Position == 0 && i > 0 {
			crmStage.Position = i
		}
		stageType := providers.InferCRMStageType(crmStage, i == len(stagesIn)-1)
		if err := ValidateStageType(stageType); err != nil {
			stageType = StageTypeStandard
		}

		st := Stage{}
		err = tx.QueryRow(ctx,
			`INSERT INTO pipeline_stages(pipeline_id, name, position, color, stage_type)
			 VALUES ($1, $2, $3, 'gray', $4)
			 RETURNING id, public_id, pipeline_id, name, position, color, stage_type, created_at`,
			pl.ID, stName, i, stageType).Scan(
			&st.ID, &st.PublicID, &st.PipelineID, &st.Name, &st.Position, &st.Color, &st.StageType, &st.CreatedAt)
		if err != nil {
			return nil, nil, err
		}
		stages = append(stages, st)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return pl, stages, nil
}
