package core_telegram_tgctx

import (
	"strings"

	"github.com/ra1phdd/logger"
	tele "gopkg.in/telebot.v4"
)

const requestIDKey = "request_id"
const loggerKey = "logger"

func MessageAttrs(c tele.Context) []logger.Attr {
	attrs := make([]logger.Attr, 0, 9)
	if c == nil {
		return attrs
	}

	attrs = append(attrs,
		logger.Int("update_id", c.Update().ID),
		logger.String("event", EventType(c)),
	)

	if command := Command(c); command != "" {
		attrs = append(attrs, logger.String("command", command))
	}

	if chat := c.Chat(); chat != nil {
		attrs = append(attrs,
			logger.Int64("chat_id", chat.ID),
			logger.String("chat_type", string(chat.Type)),
		)
	}

	if msg := c.Message(); msg != nil {
		attrs = append(attrs, logger.Int("message_id", msg.ID))
	}

	if sender := Sender(c); sender != nil {
		attrs = append(attrs,
			logger.Int64("sender_id", sender.ID),
			logger.String("sender_username", sender.Username),
		)
	}

	if requestID, ok := requestIDFromContext(c); ok {
		attrs = append(attrs, logger.String("request_id", requestID))
	}

	return attrs
}

func Logger(c tele.Context, fallback *logger.Logger) *logger.Logger {
	if log := loggerFromContext(c); log != nil {
		return log
	}
	if fallback != nil {
		return fallback.With(MessageAttrs(c)...)
	}
	return logger.New(logger.WithComponent("telegram.context")).With(MessageAttrs(c)...)
}

func requestIDFromContext(c tele.Context) (string, bool) {
	if c == nil {
		return "", false
	}

	requestID, ok := c.Get(requestIDKey).(string)
	return requestID, ok
}

func loggerFromContext(c tele.Context) *logger.Logger {
	if c == nil {
		return nil
	}

	log, _ := c.Get(loggerKey).(*logger.Logger)
	return log
}

func ReplyText(c tele.Context) string {
	if c == nil || c.Message() == nil || c.Message().ReplyTo == nil {
		return ""
	}

	reply := c.Message().ReplyTo
	return strings.TrimSpace(reply.Text + " " + reply.Caption)
}

func NormalizeCallbackData(data string) string {
	data = strings.TrimSpace(data)
	return strings.TrimPrefix(data, "\f")
}

func SenderID(c tele.Context) int64 {
	sender := Sender(c)
	if sender == nil {
		return 0
	}

	return sender.ID
}

func Sender(c tele.Context) *tele.User {
	if c == nil {
		return nil
	}

	if msg := c.Message(); msg != nil && msg.Sender != nil {
		return msg.Sender
	}

	return c.Sender()
}

func Command(c tele.Context) string {
	if c == nil {
		return ""
	}

	text := strings.TrimSpace(c.Text())
	if !strings.HasPrefix(text, "/") {
		return ""
	}

	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

func EventType(c tele.Context) string {
	if c == nil {
		return "unknown"
	}

	switch {
	case c.Callback() != nil:
		return "callback"
	case c.Query() != nil:
		return "inline_query"
	case c.ChatMember() != nil:
		return "chat_member"
	case c.ChatJoinRequest() != nil:
		return "chat_join_request"
	case c.Poll() != nil:
		return "poll"
	case c.PollAnswer() != nil:
		return "poll_answer"
	case c.Message() != nil:
		return "message"
	default:
		return "update"
	}
}
