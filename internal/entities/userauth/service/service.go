package userauth_service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	"getytstatsapi/internal/core/security/jwtutil"
	"getytstatsapi/internal/core/security/telegramauth"

	"github.com/google/uuid"
)

type SessionStore interface {
	CreatePublicSession(context.Context, domain.PublicUserSession) error
	GetPublicSession(context.Context, string) (domain.PublicUserSession, error)
	UpdatePublicSession(context.Context, string, string, time.Time) error
	RevokePublicSession(context.Context, string) error
}

type Service struct {
	store            SessionStore
	telegramBotToken string
	accessSecret     string
	refreshSecret    string
	accessTTL        time.Duration
	refreshTTL       time.Duration
	now              func() time.Time
}

type LoginRequest struct {
	Mode     string
	Widget   telegramauth.LoginWidgetPayload
	InitData string
}

type SessionPair struct {
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
	ExpiresIn    int64               `json:"expires_in"`
	User         telegramauth.Result `json:"user"`
}

type AccessClaims struct {
	Subject   int64  `json:"sub"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	PhotoURL  string `json:"photo_url,omitempty"`
	ExpiresAt int64  `json:"exp"`
}

type RefreshClaims struct {
	SessionID string `json:"sid"`
	Subject   int64  `json:"sub"`
	ExpiresAt int64  `json:"exp"`
}

func New(store SessionStore, telegramBotToken string, accessSecret string, refreshSecret string) *Service {
	return &Service{
		store:            store,
		telegramBotToken: strings.TrimSpace(telegramBotToken),
		accessSecret:     strings.TrimSpace(accessSecret),
		refreshSecret:    strings.TrimSpace(refreshSecret),
		accessTTL:        domain.DefaultAccessTokenLifetime,
		refreshTTL:       domain.DefaultRefreshTokenLifetime,
		now:              time.Now,
	}
}

func (s *Service) Login(ctx context.Context, request LoginRequest) (SessionPair, error) {
	var (
		user telegramauth.Result
		err  error
	)

	switch strings.ToLower(strings.TrimSpace(request.Mode)) {
	case "widget":
		user, err = telegramauth.ValidateLoginWidget(request.Widget, s.telegramBotToken, 24*time.Hour)
	case "webapp":
		user, err = telegramauth.ValidateWebAppInitData(request.InitData, s.telegramBotToken, 24*time.Hour)
	default:
		return SessionPair{}, fmt.Errorf("%w: unsupported telegram auth mode", core_errors.ErrInvalidArgument)
	}
	if err != nil {
		return SessionPair{}, fmt.Errorf("%w: %w", core_errors.ErrUnauthorized, err)
	}

	return s.issueSessionPair(ctx, user, "")
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (SessionPair, error) {
	claims, err := s.ParseRefreshToken(refreshToken)
	if err != nil {
		return SessionPair{}, err
	}

	session, err := s.store.GetPublicSession(ctx, claims.SessionID)
	if err != nil {
		return SessionPair{}, fmt.Errorf("%w: %w", core_errors.ErrUnauthorized, err)
	}
	if session.RevokedAt != nil || session.ExpiresAt.Before(s.now().UTC()) {
		return SessionPair{}, fmt.Errorf("%w: refresh session expired", core_errors.ErrUnauthorized)
	}
	if session.TelegramUserID != claims.Subject {
		return SessionPair{}, fmt.Errorf("%w: refresh token subject mismatch", core_errors.ErrUnauthorized)
	}
	if session.RefreshTokenHash != hashToken(refreshToken) {
		return SessionPair{}, fmt.Errorf("%w: refresh token mismatch", core_errors.ErrUnauthorized)
	}

	return s.issueSessionPair(ctx, telegramauth.Result{UserID: claims.Subject}, session.ID)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.ParseRefreshToken(refreshToken)
	if err != nil {
		return err
	}
	if err := s.store.RevokePublicSession(ctx, claims.SessionID); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

func (s *Service) ParseAccessToken(token string) (AccessClaims, error) {
	var claims AccessClaims
	if err := jwtutil.ParseHS256(token, s.accessSecret, &claims); err != nil {
		return AccessClaims{}, fmt.Errorf("%w: invalid access token", core_errors.ErrUnauthorized)
	}
	if claims.Subject == 0 || claims.ExpiresAt == 0 {
		return AccessClaims{}, fmt.Errorf("%w: invalid access token claims", core_errors.ErrUnauthorized)
	}
	if time.Unix(claims.ExpiresAt, 0).Before(s.now()) {
		return AccessClaims{}, fmt.Errorf("%w: access token expired", core_errors.ErrUnauthorized)
	}
	return claims, nil
}

func (s *Service) ParseRefreshToken(token string) (RefreshClaims, error) {
	var claims RefreshClaims
	if err := jwtutil.ParseHS256(token, s.refreshSecret, &claims); err != nil {
		return RefreshClaims{}, fmt.Errorf("%w: invalid refresh token", core_errors.ErrUnauthorized)
	}
	if claims.Subject == 0 || claims.SessionID == "" || claims.ExpiresAt == 0 {
		return RefreshClaims{}, fmt.Errorf("%w: invalid refresh token claims", core_errors.ErrUnauthorized)
	}
	if time.Unix(claims.ExpiresAt, 0).Before(s.now()) {
		return RefreshClaims{}, fmt.Errorf("%w: refresh token expired", core_errors.ErrUnauthorized)
	}
	return claims, nil
}

func (s *Service) issueSessionPair(ctx context.Context, user telegramauth.Result, existingSessionID string) (SessionPair, error) {
	now := s.now().UTC()
	accessClaims := AccessClaims{
		Subject:   user.UserID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		PhotoURL:  user.PhotoURL,
		ExpiresAt: now.Add(s.accessTTL).Unix(),
	}
	accessToken, err := jwtutil.SignHS256(accessClaims, s.accessSecret)
	if err != nil {
		return SessionPair{}, fmt.Errorf("sign access token: %w", err)
	}

	sessionID := existingSessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	refreshClaims := RefreshClaims{
		SessionID: sessionID,
		Subject:   user.UserID,
		ExpiresAt: now.Add(s.refreshTTL).Unix(),
	}
	refreshToken, err := jwtutil.SignHS256(refreshClaims, s.refreshSecret)
	if err != nil {
		return SessionPair{}, fmt.Errorf("sign refresh token: %w", err)
	}

	refreshHash := hashToken(refreshToken)
	if existingSessionID == "" {
		err = s.store.CreatePublicSession(ctx, domain.PublicUserSession{
			ID:               sessionID,
			TelegramUserID:   user.UserID,
			RefreshTokenHash: refreshHash,
			ExpiresAt:        now.Add(s.refreshTTL),
		})
	} else {
		err = s.store.UpdatePublicSession(ctx, sessionID, refreshHash, now.Add(s.refreshTTL))
	}
	if err != nil {
		return SessionPair{}, fmt.Errorf("persist public session: %w", err)
	}

	return SessionPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL / time.Second),
		User:         user,
	}, nil
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}
