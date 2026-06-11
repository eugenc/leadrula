package contracts

import (
	"net/http"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func (h *Handler) buyerListParticipationCompensations(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListCompensationsForParticipationBuyer(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Compensation{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerPatchContractCompensation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		TriggerStageID int64 `json:"trigger_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateBuyerCompensationTriggerStage(r.Context(), p.AccountID, idp(r, "id"), idp(r, "compId"), body.TriggerStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) buyerPatchParticipationCompensation(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		TriggerStageID int64 `json:"trigger_stage_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateParticipationCompensationTriggerStage(r.Context(), p.AccountID, idp(r, "id"), idp(r, "compId"), body.TriggerStageID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) buyerListContractFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListFieldMapForBuyer(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []FieldMapEntry{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerListParticipationFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListFieldMapForParticipationBuyer(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []FieldMapEntry{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) buyerContractFieldMapOptions(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	opts, err := h.svc.FieldMapOptionsForBuyer(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, opts)
}

func (h *Handler) buyerParticipationFieldMapOptions(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	opts, err := h.svc.FieldMapOptionsForParticipationBuyer(r.Context(), p.AccountID, idp(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, opts)
}

func (h *Handler) buyerAddContractFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	params, ok := decodeFieldMapBody(w, r)
	if !ok {
		return
	}
	e, err := h.svc.AddFieldMapForBuyer(r.Context(), p.AccountID, idp(r, "id"), params)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) buyerAddParticipationFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	params, ok := decodeFieldMapBody(w, r)
	if !ok {
		return
	}
	e, err := h.svc.AddFieldMapForParticipationBuyer(r.Context(), p.AccountID, idp(r, "id"), params)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) buyerDeleteContractFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteFieldMapForBuyer(r.Context(), p.AccountID, idp(r, "id"), idp(r, "mapId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) buyerDeleteParticipationFieldMap(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteFieldMapForParticipationBuyer(r.Context(), p.AccountID, idp(r, "id"), idp(r, "mapId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func decodeFieldMapBody(w http.ResponseWriter, r *http.Request) (AddFieldMapParams, bool) {
	var body struct {
		SrcType          string `json:"src_type"`
		SrcBuiltin       string `json:"src_builtin"`
		SrcCustomFieldID *int64 `json:"src_custom_field_id"`
		DstType          string `json:"dst_type"`
		DstBuiltin       string `json:"dst_builtin"`
		DstCustomFieldID *int64 `json:"dst_custom_field_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return AddFieldMapParams{}, false
	}
	return AddFieldMapParams{
		SrcType:          body.SrcType,
		SrcBuiltin:       body.SrcBuiltin,
		SrcCustomFieldID: body.SrcCustomFieldID,
		DstType:          body.DstType,
		DstBuiltin:       body.DstBuiltin,
		DstCustomFieldID: body.DstCustomFieldID,
	}, true
}
