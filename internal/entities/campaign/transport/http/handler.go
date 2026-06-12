package campaign_http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	"getytstatsapi/internal/core/security/serviceauth"
	"getytstatsapi/internal/core/security/telegramauth"
	core_http_response "getytstatsapi/internal/core/transport/http/response"
	campaign_service "getytstatsapi/internal/entities/campaign/service"
	userauth_service "getytstatsapi/internal/entities/userauth/service"

	"github.com/ra1phdd/logger"
)

const dateLayout = "2006-01-02"

type Handler struct {
	log           *logger.Logger
	campaigns     *campaign_service.Service
	auth          *userauth_service.Service
	publicBaseURL string
	peerServiceID string
	peerSecret    string
	nonces        *serviceauth.NonceStore
}

func NewHandler(log *logger.Logger, campaigns *campaign_service.Service, auth *userauth_service.Service, publicBaseURL string, peerServiceID string, peerSecret string) *Handler {
	if log == nil {
		log = logger.New(logger.WithComponent("campaign.http"))
	}
	return &Handler{
		log:           log,
		campaigns:     campaigns,
		auth:          auth,
		publicBaseURL: strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"),
		peerServiceID: strings.TrimSpace(peerServiceID),
		peerSecret:    strings.TrimSpace(peerSecret),
		nonces:        serviceauth.NewNonceStore(5 * time.Minute),
	}
}

func (h *Handler) AuthTelegram(w http.ResponseWriter, r *http.Request) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		Mode     string `json:"mode"`
		InitData string `json:"init_data"`
		Payload  struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
			PhotoURL  string `json:"photo_url"`
			AuthDate  int64  `json:"auth_date"`
			Hash      string `json:"hash"`
		} `json:"widget"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid auth request", err)
		return
	}

	session, err := h.auth.Login(r.Context(), userauth_service.LoginRequest{
		Mode:     request.Mode,
		InitData: request.InitData,
		Widget: telegramauth.LoginWidgetPayload{
			ID:        request.Payload.ID,
			FirstName: request.Payload.FirstName,
			LastName:  request.Payload.LastName,
			Username:  request.Payload.Username,
			PhotoURL:  request.Payload.PhotoURL,
			AuthDate:  request.Payload.AuthDate,
			Hash:      request.Payload.Hash,
		},
	})
	if err != nil {
		response.ErrorResponse("telegram auth failed", err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) AuthRefresh(w http.ResponseWriter, r *http.Request) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid refresh request", err)
		return
	}
	session, err := h.auth.Refresh(r.Context(), request.RefreshToken)
	if err != nil {
		response.ErrorResponse("refresh failed", err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) AuthLogout(w http.ResponseWriter, r *http.Request) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid logout request", err)
		return
	}
	if err := h.auth.Logout(r.Context(), request.RefreshToken); err != nil {
		response.ErrorResponse("logout failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	h.withUserAuth(func(w http.ResponseWriter, r *http.Request, claims userauth_service.AccessClaims) {
		settings, err := h.campaigns.GetUserSettings(r.Context(), claims.Subject)
		if err != nil {
			core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w).ErrorResponse("failed to get profile", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"telegram_user_id": claims.Subject,
				"username":         claims.Username,
				"first_name":       claims.FirstName,
				"last_name":        claims.LastName,
				"photo_url":        claims.PhotoURL,
			},
			"settings": settings,
		})
	})(w, r)
}

func (h *Handler) GetUserChannels(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.listChannels(w, r, userID)
	})(w, r)
}

func (h *Handler) CreateUserChannel(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.createChannel(w, r, userID)
	})(w, r)
}

func (h *Handler) ResolveUserChannel(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.resolveChannel(w, r, userID)
	})(w, r)
}

func (h *Handler) VerifyUserChannel(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.verifyChannel(w, r, userID)
	})(w, r)
}

func (h *Handler) GetGoogleLink(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
		url, err := h.campaigns.GetGoogleLinkURL(userID)
		if err != nil {
			response.ErrorResponse("failed to build google link", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"url": url})
	})(w, r)
}

func (h *Handler) CompleteGoogleLink(w http.ResponseWriter, r *http.Request) {
	settings, err := h.campaigns.CompleteGoogleLink(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if err != nil {
		logger.FromContext(r.Context()).Warn("google link callback failed", logger.Err(err))
		writeText(w, http.StatusOK, "Не удалось подключить Google аккаунт. Вернитесь в Telegram и попробуйте еще раз.")
		return
	}
	message := "Google аккаунт подключен."
	if strings.TrimSpace(settings.GoogleEmail) != "" {
		message = "Google аккаунт подключен: " + settings.GoogleEmail
	}
	writeText(w, http.StatusOK, message+" Вернитесь в Telegram и откройте раздел Google в настройках.")
}

func (h *Handler) CreateCampaignSpreadsheet(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
		campaignID, err := parseInt64Path(r, "campaign_id")
		if err != nil {
			response.ErrorResponse("invalid campaign id", err)
			return
		}
		item, err := h.campaigns.CreateCampaignSpreadsheet(r.Context(), userID, campaignID)
		if err != nil {
			response.ErrorResponse("failed to create campaign spreadsheet", err)
			return
		}
		writeJSON(w, http.StatusOK, h.campaignToResponse(item))
	})(w, r)
}

func (h *Handler) DeleteUserChannel(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.deleteChannel(w, r, userID)
	})(w, r)
}

func (h *Handler) GetUserCampaigns(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.listCampaigns(w, r, userID)
	})(w, r)
}

func (h *Handler) CreateUserCampaign(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.createCampaign(w, r, userID)
	})(w, r)
}

func (h *Handler) GetUserCampaign(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.getCampaign(w, r, userID)
	})(w, r)
}

func (h *Handler) CloseUserCampaign(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.closeCampaign(w, r, userID)
	})(w, r)
}

func (h *Handler) RefreshUserCampaign(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.refreshCampaign(w, r, userID)
	})(w, r)
}

func (h *Handler) UpdateUserCampaignColumns(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.updateCampaignColumns(w, r, userID)
	})(w, r)
}

func (h *Handler) UpdateUserCampaignTarget(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.updateCampaignTarget(w, r, userID)
	})(w, r)
}

func (h *Handler) GetUserSettings(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.getSettings(w, r, userID)
	})(w, r)
}

func (h *Handler) PatchUserSettings(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		h.patchSettings(w, r, userID)
	})(w, r)
}

func (h *Handler) ExportCampaign(w http.ResponseWriter, r *http.Request) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	token := r.PathValue("token")
	data, title, err := h.campaigns.ExportCampaignCSV(r.Context(), token)
	if err != nil {
		response.ErrorResponse("failed to export campaign", err)
		return
	}
	filename := sanitizeFilename(title)
	if filename == "" {
		filename = "campaign"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", filename))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *Handler) GetUserInputSession(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
		session, err := h.campaigns.GetInputSession(r.Context(), userID)
		if err != nil {
			response.ErrorResponse("failed to get input session", err)
			return
		}
		writeJSON(w, http.StatusOK, session)
	})(w, r)
}

func (h *Handler) PutUserInputSession(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
		var request struct {
			Flow      string     `json:"flow"`
			Step      string     `json:"step"`
			Payload   string     `json:"payload"`
			ExpiresAt *time.Time `json:"expires_at"`
		}
		if err := decodeJSON(r, &request); err != nil {
			response.ErrorResponse("invalid input session request", err)
			return
		}
		session, err := h.campaigns.UpsertInputSession(r.Context(), domain.CampaignInputSession{
			TelegramUserID: userID,
			Flow:           request.Flow,
			Step:           request.Step,
			Payload:        request.Payload,
			ExpiresAt:      request.ExpiresAt,
		})
		if err != nil {
			response.ErrorResponse("failed to upsert input session", err)
			return
		}
		writeJSON(w, http.StatusOK, session)
	})(w, r)
}

func (h *Handler) DeleteUserInputSession(w http.ResponseWriter, r *http.Request) {
	h.withActorAuth(parseUserIDPath, func(w http.ResponseWriter, r *http.Request, userID int64) {
		response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
		if err := h.campaigns.DeleteInputSession(r.Context(), userID); err != nil {
			response.ErrorResponse("failed to delete input session", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})(w, r)
}

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	items, err := h.campaigns.ListUserChannels(r.Context(), userID)
	if err != nil {
		response.ErrorResponse("failed to list channels", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		ChannelID string `json:"channel_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid create channel request", err)
		return
	}
	item, err := h.campaigns.AddUserChannel(r.Context(), userID, request.ChannelID)
	if err != nil {
		response.ErrorResponse("failed to add channel", err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) resolveChannel(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		ChannelID string `json:"channel_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid resolve channel request", err)
		return
	}
	item, err := h.campaigns.PrepareUserChannelVerification(r.Context(), userID, request.ChannelID)
	if err != nil {
		response.ErrorResponse("failed to resolve channel", err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) verifyChannel(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		ChannelID string `json:"channel_id"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid verify channel request", err)
		return
	}
	item, err := h.campaigns.VerifyAndAddUserChannel(r.Context(), userID, request.ChannelID)
	if err != nil {
		response.ErrorResponse("failed to verify channel", err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) deleteChannel(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	if err := h.campaigns.DeleteUserChannel(r.Context(), userID, r.PathValue("channel_id")); err != nil {
		response.ErrorResponse("failed to delete channel", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	items, err := h.campaigns.ListCampaigns(r.Context(), userID, r.URL.Query().Get("status"), page, pageSize)
	if err != nil {
		response.ErrorResponse("failed to list campaigns", err)
		return
	}
	responseItems := make([]map[string]any, 0, len(items.Items))
	for _, item := range items.Items {
		responseItems = append(responseItems, h.campaignToResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":     responseItems,
		"page":      items.Page,
		"page_size": items.PageSize,
		"has_prev":  items.HasPrev,
		"has_next":  items.HasNext,
		"total":     items.Total,
	})
}

func (h *Handler) createCampaign(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		ChannelID   string   `json:"channel_id"`
		Keyword     string   `json:"keyword"`
		StartDate   string   `json:"start_date"`
		TargetViews *int64   `json:"target_views"`
		Columns     []string `json:"columns"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid create campaign request", err)
		return
	}
	startDate, err := time.Parse(dateLayout, request.StartDate)
	if err != nil {
		response.ErrorResponse("invalid create campaign request", fmt.Errorf("%w: invalid start_date", core_errors.ErrInvalidArgument))
		return
	}
	columns := make([]domain.StatsColumn, 0, len(request.Columns))
	for _, item := range request.Columns {
		column := domain.StatsColumn(strings.TrimSpace(item))
		if !domain.IsValidStatsColumn(column) {
			response.ErrorResponse("invalid create campaign request", fmt.Errorf("%w: invalid column %s", core_errors.ErrInvalidArgument, item))
			return
		}
		columns = append(columns, column)
	}
	item, err := h.campaigns.CreateCampaign(r.Context(), campaign_service.CreateCampaignRequest{
		TelegramUserID: userID,
		ChannelID:      request.ChannelID,
		Keyword:        request.Keyword,
		StartDate:      startDate,
		TargetViews:    request.TargetViews,
		Columns:        columns,
	})
	if err != nil {
		response.ErrorResponse("failed to create campaign", err)
		return
	}
	writeJSON(w, http.StatusCreated, h.campaignToResponse(item))
}

func (h *Handler) getCampaign(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	campaignID, err := parseInt64Path(r, "campaign_id")
	if err != nil {
		response.ErrorResponse("invalid campaign id", err)
		return
	}
	item, err := h.campaigns.GetCampaign(r.Context(), userID, campaignID)
	if err != nil {
		response.ErrorResponse("failed to get campaign", err)
		return
	}
	writeJSON(w, http.StatusOK, h.campaignToResponse(item))
}

func (h *Handler) closeCampaign(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	campaignID, err := parseInt64Path(r, "campaign_id")
	if err != nil {
		response.ErrorResponse("invalid campaign id", err)
		return
	}
	item, err := h.campaigns.CloseCampaign(r.Context(), userID, campaignID)
	if err != nil {
		response.ErrorResponse("failed to close campaign", err)
		return
	}
	writeJSON(w, http.StatusOK, h.campaignToResponse(item))
}

func (h *Handler) refreshCampaign(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	campaignID, err := parseInt64Path(r, "campaign_id")
	if err != nil {
		response.ErrorResponse("invalid campaign id", err)
		return
	}
	item, snapshot, autoClosed, err := h.campaigns.RefreshCampaign(r.Context(), userID, campaignID)
	if err != nil {
		response.ErrorResponse("failed to refresh campaign", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaign":    h.campaignToResponse(item),
		"snapshot":    snapshot,
		"auto_closed": autoClosed,
	})
}

func (h *Handler) updateCampaignColumns(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	campaignID, err := parseInt64Path(r, "campaign_id")
	if err != nil {
		response.ErrorResponse("invalid campaign id", err)
		return
	}
	var request struct {
		Columns []string `json:"columns"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid update campaign columns request", err)
		return
	}
	columns := make([]domain.StatsColumn, 0, len(request.Columns))
	for _, item := range request.Columns {
		column := domain.StatsColumn(strings.TrimSpace(item))
		if !domain.IsValidStatsColumn(column) {
			response.ErrorResponse("invalid update campaign columns request", fmt.Errorf("%w: invalid column %s", core_errors.ErrInvalidArgument, item))
			return
		}
		columns = append(columns, column)
	}
	item, err := h.campaigns.UpdateCampaignColumns(r.Context(), userID, campaignID, columns)
	if err != nil {
		response.ErrorResponse("failed to update campaign columns", err)
		return
	}
	writeJSON(w, http.StatusOK, h.campaignToResponse(item))
}

func (h *Handler) updateCampaignTarget(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	campaignID, err := parseInt64Path(r, "campaign_id")
	if err != nil {
		response.ErrorResponse("invalid campaign id", err)
		return
	}
	var request struct {
		TargetViews *int64 `json:"target_views"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid update campaign target request", err)
		return
	}
	item, err := h.campaigns.UpdateCampaignTarget(r.Context(), userID, campaignID, request.TargetViews)
	if err != nil {
		response.ErrorResponse("failed to update campaign target", err)
		return
	}
	writeJSON(w, http.StatusOK, h.campaignToResponse(item))
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	settings, err := h.campaigns.GetUserSettings(r.Context(), userID)
	if err != nil {
		response.ErrorResponse("failed to get settings", err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) patchSettings(w http.ResponseWriter, r *http.Request, userID int64) {
	response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	var request struct {
		NotificationsEnabled        bool   `json:"notifications_enabled"`
		NotificationTime            string `json:"notification_time"`
		NotificationIntervalMinutes int    `json:"notification_interval_minutes"`
		Timezone                    string `json:"timezone"`
	}
	if err := decodeJSON(r, &request); err != nil {
		response.ErrorResponse("invalid settings request", err)
		return
	}
	settings, err := h.campaigns.UpdateUserSettings(r.Context(), domain.UserSettings{
		TelegramUserID:              userID,
		NotificationsEnabled:        request.NotificationsEnabled,
		NotificationTime:            request.NotificationTime,
		NotificationIntervalMinutes: request.NotificationIntervalMinutes,
		Timezone:                    request.Timezone,
	})
	if err != nil {
		response.ErrorResponse("failed to update settings", err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) withUserAuth(next func(http.ResponseWriter, *http.Request, userauth_service.AccessClaims)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
		authorization := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			response.ErrorResponse("authorization required", fmt.Errorf("%w: missing bearer token", core_errors.ErrUnauthorized))
			return
		}
		claims, err := h.auth.ParseAccessToken(strings.TrimSpace(authorization[7:]))
		if err != nil {
			response.ErrorResponse("authorization failed", err)
			return
		}
		next(w, r, claims)
	}
}

func (h *Handler) withActorAuth(extractUserID func(*http.Request) int64, next func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
		targetUserID := extractUserID(r)
		if targetUserID == 0 {
			response.ErrorResponse("invalid user id", fmt.Errorf("%w: invalid user_id", core_errors.ErrInvalidArgument))
			return
		}

		authorization := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			claims, err := h.auth.ParseAccessToken(strings.TrimSpace(authorization[7:]))
			if err != nil {
				response.ErrorResponse("authorization failed", err)
				return
			}
			if claims.Subject != targetUserID {
				response.ErrorResponse("forbidden", fmt.Errorf("%w: token user does not match target user", core_errors.ErrForbidden))
				return
			}
			next(w, r, targetUserID)
			return
		}

		if err := serviceauth.VerifyRequest(r, h.peerServiceID, h.peerSecret, h.nonces); err != nil {
			response.ErrorResponse("authorization required", fmt.Errorf("%w: request is not authorized", core_errors.ErrUnauthorized))
			return
		}
		next(w, r, targetUserID)
	}
}

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseUserIDPath(r *http.Request) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(r.PathValue("user_id")), 10, 64)
	return value
}

func parseInt64Path(r *http.Request, name string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(r.PathValue(name)), 10, 64)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("%w: invalid %s", core_errors.ErrInvalidArgument, name)
	}
	return value, nil
}

func (h *Handler) campaignToResponse(item domain.Campaign) map[string]any {
	response := map[string]any{
		"id":               item.ID,
		"telegram_user_id": item.TelegramUserID,
		"channel_id":       item.ChannelID,
		"channel_title":    item.ChannelTitle,
		"keyword":          item.Keyword,
		"start_date":       item.StartDate.Format(dateLayout),
		"timezone":         item.Timezone,
		"status":           item.Status,
		"columns":          item.Columns,
		"title":            item.Title(),
		"last_total_views": item.LastTotalViews,
		"created_at":       item.CreatedAt,
		"updated_at":       item.UpdatedAt,
		"export_url":       item.ExportURL(h.publicBaseURL),
		"spreadsheet_url":  item.SpreadsheetURL,
	}
	if item.TargetViews != nil {
		response["target_views"] = *item.TargetViews
	}
	if item.ClosedAt != nil {
		response["closed_at"] = item.ClosedAt
	}
	if item.CloseReason != "" {
		response["close_reason"] = item.CloseReason
	}
	if item.LastSnapshotAt != nil {
		response["last_snapshot_at"] = item.LastSnapshotAt
	}
	if item.LastDailyGrowth != nil {
		response["last_daily_growth"] = *item.LastDailyGrowth
	}
	if item.EstimatedCloseDate != nil {
		response["estimated_close_date"] = item.EstimatedCloseDate.Format(dateLayout)
	}
	if item.ExportJWT != "" {
		response["export_token"] = item.ExportJWT
	}
	return response
}

func writeText(w http.ResponseWriter, statusCode int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(body))
}

func sanitizeFilename(value string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(strings.TrimSpace(value))
}
