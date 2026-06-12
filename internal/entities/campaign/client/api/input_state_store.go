package campaignapi_client

import (
	"context"
	"errors"
	"strings"

	"getytstatsapi/internal/core/domain"
)

type InputStateStore struct {
	client *Client
	ctx    context.Context
}

func NewInputStateStore(ctx context.Context, client *Client) *InputStateStore {
	if ctx == nil {
		ctx = context.Background()
	}
	return &InputStateStore{client: client, ctx: ctx}
}

func (s *InputStateStore) Get(userID int64) (string, error) {
	session, err := s.client.GetInputSession(s.ctx, userID)
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return "", nil
		}
		return "", err
	}
	return session.Step, nil
}

func (s *InputStateStore) Set(userID int64, state string) error {
	session, err := s.client.GetInputSession(s.ctx, userID)
	var apiErr *Error
	if err != nil && !(errors.As(err, &apiErr) && apiErr.IsNotFound()) {
		return err
	}
	if errors.As(err, &apiErr) && apiErr.IsNotFound() {
		session = domain.CampaignInputSession{
			TelegramUserID: userID,
			Flow:           "telegram",
			Payload:        "{}",
		}
	}
	session.Step = state
	if strings.TrimSpace(session.Flow) == "" {
		session.Flow = "telegram"
	}
	if strings.TrimSpace(session.Payload) == "" {
		session.Payload = "{}"
	}
	_, err = s.client.UpsertInputSession(s.ctx, session)
	return err
}

func (s *InputStateStore) Clear(userID int64) error {
	return s.client.DeleteInputSession(s.ctx, userID)
}
