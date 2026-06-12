package core_telegram_input

import (
	"strings"
	"sync"
	"time"

	core_telegram_tgctx "getytstatsapi/internal/core/transport/telegram/tgctx"

	"github.com/ra1phdd/logger"
	tele "gopkg.in/telebot.v4"
)

const replyMarkupKey = "replyKeyboard"

type Telegram struct {
	log                   *logger.Logger
	mu                    sync.RWMutex
	routers               map[int64]string
	handlers              map[string]tele.HandlerFunc
	timers                map[int64]*time.Timer
	unknownCommandMessage string
}

func NewInput(log *logger.Logger) *Telegram {
	if log == nil {
		log = logger.New(logger.WithComponent("telegram.input"))
	}

	return &Telegram{
		log:                   log,
		routers:               make(map[int64]string),
		handlers:              make(map[string]tele.HandlerFunc),
		timers:                make(map[int64]*time.Timer),
		unknownCommandMessage: "🤔 Неизвестная команда. Используйте /start для работы с ботом",
	}
}

func (t *Telegram) RegisterHandler(state string, handler tele.HandlerFunc) {
	if t == nil || handler == nil {
		return
	}

	state = strings.TrimSpace(state)
	if state == "" {
		return
	}

	t.handlers[state] = handler
}

func (t *Telegram) Set(userID int64, state string) {
	t.SetWithTTL(userID, state, 0, nil)
}

func (t *Telegram) SetWithTTL(userID int64, state string, ttl time.Duration, onExpired func()) {
	if t == nil || userID == 0 {
		return
	}

	state = strings.TrimSpace(state)

	t.mu.Lock()
	defer t.mu.Unlock()

	t.routers[userID] = state
	if timer, exists := t.timers[userID]; exists {
		timer.Stop()
		delete(t.timers, userID)
	}

	if ttl > 0 {
		timer := time.AfterFunc(ttl, func() {
			t.mu.Lock()
			defer t.mu.Unlock()

			if currentState, ok := t.routers[userID]; ok && currentState == state {
				if onExpired != nil {
					onExpired()
				}
				t.clearUnsafe(userID)
			}
		})
		t.timers[userID] = timer
	}

	if state == "" {
		t.clearUnsafe(userID)
	}
}

func (t *Telegram) Clear(userID int64) {
	if t == nil || userID == 0 {
		return
	}

	t.mu.Lock()
	if t.routers[userID] == "" {
		t.mu.Unlock()
		return
	}
	t.clearUnsafe(userID)
	t.mu.Unlock()
}

func (t *Telegram) Has(userID int64) bool {
	if t == nil || userID == 0 {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.routers[userID] != ""
}

func (t *Telegram) SetUnknownCommandMessage(message string) {
	if t == nil {
		return
	}

	t.unknownCommandMessage = strings.TrimSpace(message)
}

func (t *Telegram) Handle(c tele.Context) error {
	if t == nil || c == nil {
		return nil
	}

	log := core_telegram_tgctx.Logger(c, t.log)
	userID := core_telegram_tgctx.SenderID(c)

	t.mu.RLock()
	state, ok := t.routers[userID]
	t.mu.RUnlock()

	if ok && state != "" {
		t.Clear(userID)
		if handler, exists := t.handlers[state]; exists {
			return handler(c)
		}

		return t.handleUnknownCommand(c, log)
	}

	return t.handleUnknownCommand(c, log)
}

func SetReplyMarkup(c tele.Context, markup *tele.ReplyMarkup) {
	if c == nil || markup == nil {
		return
	}

	c.Set(replyMarkupKey, markup)
}

func ReplyMarkupFromContext(c tele.Context) (*tele.ReplyMarkup, bool) {
	if c == nil {
		return nil, false
	}

	markup, ok := c.Get(replyMarkupKey).(*tele.ReplyMarkup)
	return markup, ok
}

func Payload(c tele.Context) string {
	if c == nil {
		return ""
	}

	return PayloadText(c.Text())
}

func PayloadText(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) < 2 {
		return ""
	}

	return strings.Join(fields[1:], " ")
}

func Args(text string) []string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) < 2 {
		return nil
	}

	return fields[1:]
}

func (t *Telegram) handleUnknownCommand(c tele.Context, log *logger.Logger) error {
	if t.unknownCommandMessage == "" {
		return nil
	}

	if markup, ok := ReplyMarkupFromContext(c); ok {
		if err := c.Send(t.unknownCommandMessage, markup); err != nil {
			log.Error("Failed to send message", logger.Err(err), logger.String("message", t.unknownCommandMessage))
			return err
		}

		return nil
	}

	if err := c.Send(t.unknownCommandMessage); err != nil {
		log.Error("Failed to send message", logger.Err(err), logger.String("message", t.unknownCommandMessage))
		return err
	}

	return nil
}

func (t *Telegram) clearUnsafe(userID int64) {
	delete(t.routers, userID)
	if timer, exists := t.timers[userID]; exists {
		timer.Stop()
		delete(t.timers, userID)
	}
}
