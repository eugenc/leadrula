package accounts

import (
	"net/http"
	"strconv"

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
