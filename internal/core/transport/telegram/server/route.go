package core_telegram_server

import tele "gopkg.in/telebot.v4"

type Route struct {
	Command string
	Handler tele.HandlerFunc
}

func NewRoute(command string, handler tele.HandlerFunc) Route {
	return Route{
		Command: command,
		Handler: handler,
	}
}
