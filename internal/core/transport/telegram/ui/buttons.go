package core_telegram_ui

import tele "gopkg.in/telebot.v4"

type Button struct {
	Text string
	Data string
	URL  string
}

func Btn(text, data string) Button {
	return Button{Text: text, Data: data}
}

func Row(buttons ...Button) []Button {
	return buttons
}

func InlineMenu(rows ...[]Button) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menuRows := make([]tele.Row, 0, len(rows))

	for _, row := range rows {
		menuRow := make(tele.Row, 0, len(row))

		for _, btn := range row {
			if btn.URL != "" {
				menuRow = append(menuRow, menu.URL(btn.Text, btn.URL))
			} else {
				menuRow = append(menuRow, menu.Data(btn.Text, "", btn.Data))
			}
		}

		menuRows = append(menuRows, menuRow)
	}

	menu.Inline(menuRows...)
	return menu
}

func CommandsKeyboard(commands ...string) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{ResizeKeyboard: true}
	if len(commands) == 0 {
		return markup
	}

	buttons := make([]tele.Btn, 0, len(commands))
	for _, command := range commands {
		if command == "" {
			continue
		}
		buttons = append(buttons, markup.Text(command))
	}

	if len(buttons) == 0 {
		return markup
	}

	markup.Reply(markup.Split(2, buttons)...)
	return markup
}

func InlineURLButton(text string, url string) *tele.ReplyMarkup {
	if text == "" || url == "" {
		return &tele.ReplyMarkup{}
	}

	return InlineMenu(Row(Button{Text: text, URL: url}))
}

func RemoveKeyboard() *tele.ReplyMarkup {
	return &tele.ReplyMarkup{RemoveKeyboard: true}
}
