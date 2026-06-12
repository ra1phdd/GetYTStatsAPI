package start_telegram

import (
	core_telegram_input "getytstatsapi/internal/core/transport/telegram/input"
	core_telegram_tgctx "getytstatsapi/internal/core/transport/telegram/tgctx"
	core_telegram_ui "getytstatsapi/internal/core/transport/telegram/ui"

	"github.com/ra1phdd/logger"
	tele "gopkg.in/telebot.v4"
)

const startMessage = "Бот подключен. Пока доступна только команда /start."

type Handler struct {
	log *logger.Logger
}

func NewHandler(log *logger.Logger) *Handler {
	if log == nil {
		log = logger.New(logger.WithComponent("start.telegram.handler"))
	}

	return &Handler{log: log}
}

func (h *Handler) Start(c tele.Context) error {
	if c == nil {
		return nil
	}

	log := core_telegram_tgctx.Logger(c, h.log)
	markup := core_telegram_ui.CommandsKeyboard("/start")
	core_telegram_input.SetReplyMarkup(c, markup)

	log.Info("telegram /start command received")
	return c.Send(startMessage, markup)
}
