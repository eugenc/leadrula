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
}

// RegisterUserRoutes mounts user/invite management for an account namespace.
func (h *Handler) RegisterUserRoutes(r chi.Router) {
	r.Get("/users", h.listUsers)
	r.With(auth.RequireRole("admin")).Post("/users/invite", h.invite)
	r.With(auth.RequireRole("admin")).Patch("/users/{id}", h.updateUser)
	r.With(auth.RequireRole("admin")).Delete("/users/{id}", h.deleteUser)
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
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	inv, err := h.svc.Invite(r.Context(), p.AccountID, body.Email, body.Role)
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
		IsActive *bool   `json:"is_active"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	u, err := h.svc.UpdateUser(r.Context(), p.AccountID, id, body.Role, body.IsActive)
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
	if err := h.svc.DeleteUser(r.Context(), p.AccountID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
