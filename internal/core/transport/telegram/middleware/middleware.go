package core_telegram_middleware

import tele "gopkg.in/telebot.v4"

type Middleware func(tele.HandlerFunc) tele.HandlerFunc

func ChainMiddleware(h tele.HandlerFunc, m ...Middleware) tele.HandlerFunc {
	if len(m) == 0 {
		return h
	}

	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}

	return h
}
