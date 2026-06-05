package accounts

import (
	"net/http"
	"strconv"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterAuthRoutes mounts the public auth endpoints.
func (h *Handler) RegisterAuthRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", h.login)
		r.Post("/refresh", h.refresh)
		r.Post("/logout", h.logout)
		r.Post("/password-reset/request", h.resetRequest)
		r.Post("/password-reset/confirm", h.resetConfirm)
		r.Post("/invite/accept", h.inviteAccept)
	})
}

// RegisterMeRoute mounts /auth/me behind RequireAuth (caller applies mw).
func (h *Handler) RegisterMeRoute(r chi.Router) {
	r.Get("/auth/me", h.me)
	r.Patch("/auth/me/prefs", h.patchPrefs)
	r.Post("/auth/me/avatar", h.uploadMyAvatar)
}

// RegisterSwitchRoutes mounts account switching endpoints.
func (h *Handler) RegisterSwitchRoutes(r chi.Router) {
	r.Get("/auth/switchable", h.listSwitchable)
	r.Post("/auth/switch", h.switchAccount)
	r.Post("/auth/switch-back", h.switchBack)
}

// RegisterPlatformRoutes mounts platform operator endpoints.
func (h *Handler) RegisterPlatformRoutes(r chi.Router) {
	r.With(auth.RequireRole("admin")).Post("/publishers", h.createPublisher)
	r.Get("/publishers", h.listPublishers)
	r.With(auth.RequireRole("admin")).Patch("/publishers/{accountId}", h.patchPublisherStatus)
	r.With(auth.RequireRole("admin")).Delete("/publishers/{accountId}", h.deletePublisher)
	r.With(auth.RequireRole("admin")).Post("/buyers", h.createPlatformBuyer)
	r.Get("/buyers", h.listAllBuyers)
	r.With(auth.RequireRole("admin")).Patch("/buyers/{accountId}", h.patchBuyerStatus)
	r.With(auth.RequireRole("admin")).Delete("/buyers/{accountId}", h.deleteBuyer)
	r.Get("/accounts/switch-log", h.switchLog)
	h.RegisterUserRoutes(r)
}

// RegisterUserRoutes mounts user/invite management for an account namespace.
func (h *Handler) RegisterUserRoutes(r chi.Router) {
	r.Get("/users", h.listUsers)
	r.With(auth.RequireRole("admin")).Post("/users/invite", h.invite)
	r.With(auth.RequireRole("admin")).Patch("/users/invites/{inviteId}", h.updateInvite)
	r.With(auth.RequireRole("admin")).Delete("/users/invites/{inviteId}", h.deleteInvite)
	r.With(auth.RequireRole("admin")).Post("/users/invites/{inviteId}/resend", h.resendInvite)
	r.With(auth.RequireRole("admin")).Patch("/users/{id}", h.updateUser)
	r.With(auth.RequireRole("admin")).Delete("/users/{id}", h.deleteUser)
	r.With(auth.RequireRole("admin")).Post("/users/{id}/avatar", h.uploadUserAvatar)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Refresh string `json:"refresh"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.Refresh(r.Context(), body.Refresh)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	// Stateless JWT: client discards tokens. Endpoint exists for symmetry.
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) resetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	_ = h.svc.RequestPasswordReset(r.Context(), body.Email)
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) resetConfirm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ConfirmPasswordReset(r.Context(), body.Token, body.NewPassword); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) inviteAccept(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		FullName string `json:"full_name"`
		Password string `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.AcceptInvite(r.Context(), body.Token, body.FullName, body.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	res, err := h.svc.Me(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) patchPrefs(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body map[string]any
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	prefs, err := h.svc.PatchPrefs(r.Context(), p.UserID, body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, prefs)
}

func (h *Handler) uploadMyAvatar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	h.uploadAvatar(w, r, p.AccountID, p.UserID)
}

func (h *Handler) uploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid user id")
		return
	}
	h.uploadAvatar(w, r, p.AccountID, id)
}

func (h *Handler) uploadAvatar(w http.ResponseWriter, r *http.Request, accountID, userID int64) {
	file, hdr, err := r.FormFile("avatar")
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "avatar file is required")
		return
	}
	defer file.Close()
	url, err := h.svc.UploadAvatar(r.Context(), accountID, userID, hdr.Header.Get("Content-Type"), file)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"avatar_url": url})
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	users, err := h.svc.ListUsers(r.Context(), p.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, users)
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Role     string `json:"role"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	inv, err := h.svc.Invite(r.Context(), p.AccountID, body.Email, body.FullName, body.Role)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, inv)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid user id")
		return
	}
	var body struct {
		Role     *string `json:"role"`
		FullName *string `json:"full_name"`
		Email    *string `json:"email"`
		IsActive *bool   `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	u, err := h.svc.UpdateUser(r.Context(), p.AccountID, id, UpdateUserParams{
		Role: body.Role, FullName: body.FullName, Email: body.Email, IsActive: body.IsActive,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid user id")
		return
	}
	if err := h.svc.DeleteUser(r.Context(), p.AccountID, p.UserID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) updateInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	inviteID, err := strconv.ParseInt(chi.URLParam(r, "inviteId"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid invite id")
		return
	}
	var body struct {
		FullName *string `json:"full_name"`
		Email    *string `json:"email"`
		Role     *string `json:"role"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	item, err := h.svc.UpdateInvite(r.Context(), p.AccountID, inviteID, UpdateInviteParams{
		FullName: body.FullName, Email: body.Email, Role: body.Role,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, item)
}

func (h *Handler) deleteInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	inviteID, err := strconv.ParseInt(chi.URLParam(r, "inviteId"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid invite id")
		return
	}
	if err := h.svc.DeleteInvite(r.Context(), p.AccountID, inviteID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listSwitchable(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListSwitchable(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) switchAccount(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		AccountID string `json:"account_id"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.SwitchAccount(r.Context(), p, body.AccountID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) switchBack(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	res, err := h.svc.SwitchBack(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) createPublisher(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string `json:"name"`
		Website        string `json:"website"`
		Timezone       string `json:"timezone"`
		AdminEmail     string `json:"admin_email"`
		AdminFirstName string `json:"admin_first_name"`
		AdminLastName  string `json:"admin_last_name"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	pub, err := h.svc.CreatePublisher(r.Context(), CreatePublisherParams{
		Name:           body.Name,
		Website:        body.Website,
		Timezone:       body.Timezone,
		AdminEmail:     body.AdminEmail,
		AdminFirstName: body.AdminFirstName,
		AdminLastName:  body.AdminLastName,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, pub)
}

func (h *Handler) listPublishers(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.repo.ListAccountsPage(r.Context(), accountListParams(r, "publisher"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) listAllBuyers(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.repo.ListAccountsPage(r.Context(), accountListParams(r, "buyer"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) patchPublisherStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name              *string `json:"name"`
		Timezone          *string `json:"timezone"`
		OperationalStatus *string `json:"operational_status"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Name == nil && body.Timezone == nil && body.OperationalStatus == nil {
		httpx.WriteError(w, httpx.Validation("no fields to update"))
		return
	}

	publicID := chi.URLParam(r, "accountId")
	var acct *Account
	var err error

	if body.Name != nil || body.Timezone != nil {
		acct, err = h.svc.UpdatePublisher(r.Context(), publicID, UpdatePublisherParams{
			Name:     body.Name,
			Timezone: body.Timezone,
		})
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	if body.OperationalStatus != nil {
		acct, err = h.svc.SetOperationalStatus(r.Context(), publicID, "publisher", *body.OperationalStatus)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}

	httpx.JSON(w, http.StatusOK, acct)
}

func (h *Handler) deletePublisher(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SoftDeletePublisher(r.Context(), chi.URLParam(r, "accountId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) patchBuyerStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name              *string `json:"name"`
		Timezone          *string `json:"timezone"`
		Website           *string `json:"website"`
		OperationalStatus *string `json:"operational_status"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Name == nil && body.Timezone == nil && body.Website == nil && body.OperationalStatus == nil {
		httpx.WriteError(w, httpx.Validation("no fields to update"))
		return
	}

	publicID := chi.URLParam(r, "accountId")
	var acct *Account
	var err error

	if body.Name != nil || body.Timezone != nil || body.Website != nil {
		acct, err = h.svc.UpdateBuyerByPublicID(r.Context(), publicID, UpdateBuyerParams{
			Name:     body.Name,
			Website:  body.Website,
			Timezone: body.Timezone,
		})
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	if body.OperationalStatus != nil {
		acct, err = h.svc.SetOperationalStatus(r.Context(), publicID, "buyer", *body.OperationalStatus)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
	}

	httpx.JSON(w, http.StatusOK, acct)
}

func (h *Handler) deleteBuyer(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SoftDeleteBuyer(r.Context(), chi.URLParam(r, "accountId")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) patchAccountStatus(w http.ResponseWriter, r *http.Request, accountType string) {
	var body struct {
		OperationalStatus string `json:"operational_status"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	a, err := h.svc.SetOperationalStatus(r.Context(), chi.URLParam(r, "accountId"), accountType, body.OperationalStatus)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func accountListParams(r *http.Request, accountType string) ListAccountsParams {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	return ListAccountsParams{
		AccountType: accountType,
		Search:      q.Get("q"),
		Page:        page,
		Limit:       limit,
	}
}

func (h *Handler) createPlatformBuyer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name            string  `json:"name"`
		Website         string  `json:"website"`
		Timezone        string  `json:"timezone"`
		AdminFirstName  string  `json:"admin_first_name"`
		AdminLastName   string  `json:"admin_last_name"`
		AdminEmail      string  `json:"admin_email"`
		StartingBalance float64 `json:"starting_balance"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	acct, err := h.svc.CreatePlatformBuyer(r.Context(), CreateBuyerParams{
		Name:            body.Name,
		Website:         body.Website,
		Timezone:        body.Timezone,
		AdminEmail:      body.AdminEmail,
		AdminFirstName:  body.AdminFirstName,
		AdminLastName:   body.AdminLastName,
		StartingBalance: body.StartingBalance,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, acct)
}

func (h *Handler) switchLog(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := h.svc.repo.Pool().Query(r.Context(),
		`SELECT l.id, u.email, fa.name, ta.name, l.switched_at
		 FROM account_switch_log l
		 JOIN users u ON u.id = l.actor_user_id
		 JOIN accounts fa ON fa.id = l.from_account_id
		 JOIN accounts ta ON ta.id = l.to_account_id
		 ORDER BY l.switched_at DESC LIMIT 200`)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id int64
		var email, fromName, toName string
		var switchedAt time.Time
		if err := rows.Scan(&id, &email, &fromName, &toName, &switchedAt); err != nil {
			httpx.WriteError(w, err)
			return
		}
		out = append(out, map[string]any{
			"id": id, "user": email, "from": fromName, "to": toName, "at": switchedAt,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	_ = p
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) resendInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	inviteID, err := strconv.ParseInt(chi.URLParam(r, "inviteId"), 10, 64)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, httpx.CodeValidation, "invalid invite id")
		return
	}
	if err := h.svc.ResendInvite(r.Context(), p.AccountID, inviteID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
