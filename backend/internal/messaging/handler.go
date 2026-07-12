package messaging

import (
	"context"
	"io"
	"net/http"

	"github.com/echayko/leadrula/backend/internal/auth"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// Handler exposes messaging over HTTP + WebSocket.
type Handler struct {
	svc      *Service
	hub      *Hub
	tokens   *auth.TokenManager
	resolve  auth.ClaimsLoader
	upgrader websocket.Upgrader
}

func NewHandler(svc *Service, hub *Hub, tokens *auth.TokenManager, resolve auth.ClaimsLoader, allowedOrigins []string) *Handler {
	origins := map[string]bool{}
	for _, o := range allowedOrigins {
		origins[o] = true
	}
	return &Handler{
		svc:     svc,
		hub:     hub,
		tokens:  tokens,
		resolve: resolve,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" || len(origins) == 0 {
					return true
				}
				return origins[origin]
			},
		},
	}
}

// RegisterRoutes mounts the shared /messages surface (publisher/buyer/platform).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/messages", func(m chi.Router) {
		m.Get("/threads", h.listThreads)
		m.Post("/threads", h.createDirect)
		m.Post("/threads/group", h.createGroup)
		m.Post("/threads/internal", h.createInternalDirect)
		m.Post("/threads/by-lead/{leadID}", h.openLeadThread)
		m.Post("/threads/by-contract/{contractID}", h.openContractThread)
		m.Post("/threads/support", h.openSupportThread)
		m.Get("/threads/{id}", h.getThread)
		m.Get("/threads/{id}/messages", h.getMessages)
		m.Post("/threads/{id}/messages", h.sendMessage)
		m.Post("/threads/{id}/read", h.markRead)
		m.Post("/threads/{id}/mute", h.setMuted)
		m.Post("/threads/{id}/archive", h.setArchived)
		m.Post("/threads/{id}/block", h.block)
		m.Post("/threads/{id}/unblock", h.unblock)
		m.Post("/threads/{id}/invite/accept", h.acceptInvite)
		m.Post("/threads/{id}/invite/decline", h.declineInvite)
		m.Patch("/{messageID}", h.editMessage)
		m.Delete("/{messageID}", h.deleteMessage)
		m.Get("/connect-requests", h.incomingConnects)
		m.Get("/connect-requests/sent", h.sentConnects)
		m.Post("/connect-requests/{id}/accept", h.acceptConnect)
		m.Post("/connect-requests/{id}/decline", h.declineConnect)
		m.Get("/group-invites", h.groupInvites)
		m.Post("/broadcasts", h.createBroadcast)
		m.Get("/broadcast-recipients", h.listBroadcastRecipients)
		m.Get("/attachments/{id}", h.downloadAttachment)
	})
}

// RegisterPlatformAudit mounts platform-only audit routes.
func (h *Handler) RegisterPlatformAudit(r chi.Router) {
	r.Post("/accounts/{accountID}/audit-mode", h.enableAudit)
	r.Delete("/accounts/{accountID}/audit-mode", h.disableAudit)
	r.Get("/accounts/{accountID}/threads", h.listAuditThreads)
}

func id(r *http.Request) string        { return chi.URLParam(r, "id") }
func msgID(r *http.Request) string     { return chi.URLParam(r, "messageID") }
func accountID(r *http.Request) string { return chi.URLParam(r, "accountID") }

func (h *Handler) listThreads(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	archived := r.URL.Query().Get("archived") == "true"
	threads, err := h.svc.ListThreads(r.Context(), p, archived, r.URL.Query().Get("q"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if threads == nil {
		threads = []Thread{}
	}
	httpx.JSON(w, http.StatusOK, threads)
}

func (h *Handler) createDirect(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req DirectRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	th, err := h.svc.CreateDirect(r.Context(), p, req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, th)
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req GroupRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	th, err := h.svc.CreateGroup(r.Context(), p, req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, th)
}

func (h *Handler) createInternalDirect(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req InternalDirectRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	th, err := h.svc.CreateInternalDirect(r.Context(), p, req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, th)
}

func (h *Handler) openLeadThread(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	th, err := h.svc.OpenLeadThread(r.Context(), p, chi.URLParam(r, "leadID"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, th)
}

func (h *Handler) openContractThread(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	th, err := h.svc.OpenContractThread(r.Context(), p, chi.URLParam(r, "contractID"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, th)
}

func (h *Handler) openSupportThread(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	th, err := h.svc.OpenSupportThread(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, th)
}

func (h *Handler) getThread(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	th, err := h.svc.GetThread(r.Context(), p, id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, th)
}

func (h *Handler) getMessages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	msgs, err := h.svc.GetMessages(r.Context(), p, id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if msgs == nil {
		msgs = []Message{}
	}
	httpx.JSON(w, http.StatusOK, msgs)
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req SendRequest
	var files []UploadFile
	if isMultipart(r) {
		var err error
		files, err = parseMessageUploads(r)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		req = SendRequest{
			Body:      r.FormValue("body"),
			ReplyToID: r.FormValue("reply_to_id"),
			LeadID:    r.FormValue("lead_id"),
		}
	} else if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	msg, err := h.svc.SendMessage(r.Context(), p, id(r), req, files)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, msg)
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.MarkRead(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) setMuted(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Muted bool `json:"muted"`
	}
	_ = httpx.DecodeJSON(w, r, &body)
	if err := h.svc.SetMuted(r.Context(), p, id(r), body.Muted); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) setArchived(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Archived bool `json:"archived"`
	}
	_ = httpx.DecodeJSON(w, r, &body)
	if err := h.svc.SetArchived(r.Context(), p, id(r), body.Archived); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) block(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.BlockThread(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) unblock(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.UnblockThread(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) acceptInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.AcceptGroupInvite(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) declineInvite(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeclineGroupInvite(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) editMessage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Body string `json:"body"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	msg, err := h.svc.EditMessage(r.Context(), p, msgID(r), body.Body)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, msg)
}

func (h *Handler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeleteMessage(r.Context(), p, msgID(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) incomingConnects(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListIncomingConnects(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []ConnectRequestView{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) sentConnects(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListSentConnects(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []ConnectRequestView{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) acceptConnect(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	th, err := h.svc.AcceptConnect(r.Context(), p, id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, th)
}

func (h *Handler) declineConnect(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DeclineConnect(r.Context(), p, id(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) groupInvites(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListGroupInvites(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []Thread{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) listBroadcastRecipients(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	items, err := h.svc.ListBroadcastRecipients(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if items == nil {
		items = []BroadcastRecipient{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createBroadcast(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var body struct {
		Body                string   `json:"body"`
		RecipientAccountIDs []string `json:"recipient_account_ids"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	job, err := h.svc.CreateBroadcast(r.Context(), p, body.Body, body.RecipientAccountIDs)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, job)
}

func (h *Handler) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	reader, contentType, filename, err := h.svc.MessageAttachment(r.Context(), p, id(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	defer reader.Close()
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")
	_, _ = io.Copy(w, reader)
}

func (h *Handler) enableAudit(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	count, err := h.svc.EnableAuditMode(r.Context(), p, accountID(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"threads": count})
}

func (h *Handler) disableAudit(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if err := h.svc.DisableAuditMode(r.Context(), p, accountID(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) listAuditThreads(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	threads, err := h.svc.ListAuditThreads(r.Context(), p, accountID(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if threads == nil {
		threads = []Thread{}
	}
	httpx.JSON(w, http.StatusOK, threads)
}

// WebSocket upgrades the connection after validating a JWT from ?token=.
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := h.tokens.ParseAccess(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	p, err := h.resolve(r.Context(), claims)
	if err != nil || p == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	hp, err := h.svc.forMessaging(r.Context(), p)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	p = hp

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	var name string
	_ = h.svc.pool.QueryRow(r.Context(), `SELECT COALESCE(full_name,'') FROM users WHERE id=$1`, p.UserID).Scan(&name)
	c := &client{conn: conn, accountID: p.AccountID, userID: p.UserID, userName: name, send: make(chan []byte, 32)}
	h.hub.add(c)
	go c.writePump()
	c.readPump(h.hub, func(threadPublicID string, typing bool) {
		h.handleTyping(context.Background(), p, threadPublicID, typing)
	})
}

// handleTyping fans typing state to the other members of a thread.
func (h *Handler) handleTyping(ctx context.Context, p *auth.Principal, threadPublicID string, typing bool) {
	hp, err := h.svc.forMessaging(ctx, p)
	if err != nil {
		return
	}
	p = hp
	threadID, err := h.svc.resolveThreadID(ctx, threadPublicID)
	if err != nil {
		return
	}
	if _, err := h.svc.loadMembership(ctx, p, threadID); err != nil {
		return
	}
	if typing {
		h.hub.setTyping(threadPublicID, p.UserID, threadTyperName(ctx, h.svc, p.UserID))
	} else {
		h.hub.clearTyping(threadPublicID, p.UserID)
	}
	evtType := "user_typing"
	if !typing {
		evtType = "user_stopped_typing"
	}
	payload := typingPayload(threadPublicID, p.UserID, threadTyperName(ctx, h.svc, p.UserID))
	h.svc.fanoutRaw(ctx, threadID, WSEvent{Type: evtType, ThreadID: threadPublicID, Payload: payload})
}
