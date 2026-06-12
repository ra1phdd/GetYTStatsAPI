package campaign_telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	"getytstatsapi/internal/core/security/serviceauth"
	core_telegram_input "getytstatsapi/internal/core/transport/telegram/input"
	core_telegram_tgctx "getytstatsapi/internal/core/transport/telegram/tgctx"
	core_telegram_ui "getytstatsapi/internal/core/transport/telegram/ui"
	campaignapi_client "getytstatsapi/internal/entities/campaign/client/api"
	campaign_service "getytstatsapi/internal/entities/campaign/service"

	"github.com/ra1phdd/logger"
	tele "gopkg.in/telebot.v4"
)

const (
	mainMenuCampaigns = "Рекламные кампании"
	mainMenuSettings  = "Настройки"

	flowCampaignCreate            = "campaign_create"
	stateCampaignKeyword          = "campaign_create_keyword"
	stateCampaignColumns          = "campaign_create_columns"
	stateCampaignManualTarget     = "campaign_create_manual_target"
	stateCampaignStartDate        = "campaign_create_start_date"
	stateSettingsAddChannel       = "settings_add_channel"
	stateSettingsConfirmChannel   = "settings_confirm_channel"
	stateSettingsNotificationTime = "settings_notification_time"
	stateSettingsTimezone         = "settings_timezone"
)

type Handler struct {
	log    *logger.Logger
	client *campaignapi_client.Client
	input  *core_telegram_input.Telegram
}

type campaignListOrigin struct {
	Status string
	Page   int
}

type pendingChannelVerification struct {
	ChannelID        string `json:"channel_id"`
	ChannelTitle     string `json:"channel_title"`
	VerificationCode string `json:"verification_code"`
	OriginalInput    string `json:"original_input,omitempty"`
}

func NewHandler(log *logger.Logger, client *campaignapi_client.Client, input *core_telegram_input.Telegram) *Handler {
	if log == nil {
		log = logger.New(logger.WithComponent("campaign.telegram.handler"))
	}
	if input != nil {
		input.RegisterHandler(stateCampaignKeyword, func(c tele.Context) error { return nil })
	}
	h := &Handler{log: log, client: client, input: input}
	if input != nil {
		input.RegisterHandler(stateCampaignKeyword, h.handleCampaignKeyword)
		input.RegisterHandler(stateCampaignColumns, h.handleCampaignColumnsText)
		input.RegisterHandler(stateCampaignManualTarget, h.handleCampaignManualTarget)
		input.RegisterHandler(stateCampaignStartDate, h.handleCampaignStartDate)
		input.RegisterHandler(stateSettingsAddChannel, h.handleAddChannelText)
		input.RegisterHandler(stateSettingsConfirmChannel, h.handleAddChannelText)
		input.RegisterHandler(stateSettingsNotificationTime, h.handleNotificationTimeText)
		input.RegisterHandler(stateSettingsTimezone, h.handleTimezoneText)
		input.SetUnknownCommandMessage("Используйте кнопки меню или /start")
	}
	return h
}

func (h *Handler) Start(c tele.Context) error {
	markup := core_telegram_ui.CommandsKeyboard(mainMenuCampaigns, mainMenuSettings)
	core_telegram_input.SetReplyMarkup(c, markup)
	return respond(c, "<b>Выберите раздел ниже.</b>", markup)
}

func (h *Handler) OnText(c tele.Context) error {
	if c == nil {
		return nil
	}
	text := strings.TrimSpace(c.Text())
	switch text {
	case mainMenuCampaigns:
		return h.showCampaignsMenu(c)
	case mainMenuSettings:
		return h.showSettingsMenu(c)
	default:
		return h.input.Handle(c)
	}
}

func (h *Handler) OnCallback(c tele.Context) error {
	if c == nil || c.Callback() == nil {
		return nil
	}
	defer c.Respond()
	data := core_telegram_tgctx.NormalizeCallbackData(c.Callback().Data)

	switch {
	case data == "campaigns:noop":
		return nil
	case data == "campaigns:menu":
		return h.showCampaignsMenu(c)
	case data == "campaigns:create":
		return h.beginCampaignCreate(c)
	case strings.HasPrefix(data, "campaigns:list:"):
		return h.showCampaignList(c, data)
	case strings.HasPrefix(data, "campaigns:view:"):
		return h.showCampaignDetail(c, data)
	case strings.HasPrefix(data, "campaigns:close:"):
		return h.closeCampaign(c, data)
	case strings.HasPrefix(data, "campaigns:refresh:"):
		return h.refreshCampaign(c, data)
	case strings.HasPrefix(data, "campaigns:sheet:"):
		return h.createCampaignSpreadsheet(c, data)
	case strings.HasPrefix(data, "campaigns:create:channel:"):
		return h.selectCreateChannel(c, data)
	case strings.HasPrefix(data, "campaigns:create:target:"):
		return h.selectCreateTarget(c, data)
	case strings.HasPrefix(data, "campaigns:create:columns:"):
		return h.handleCampaignColumnsAction(c, data)
	case data == "campaigns:create:start:today":
		return h.selectCreateStartDateToday(c)
	case data == "campaigns:create:confirm":
		return h.confirmCreateCampaign(c)
	case data == "campaigns:create:cancel":
		return h.cancelCreateCampaign(c)
	case data == "settings:menu":
		return h.showSettingsMenu(c)
	case data == "settings:channels":
		return h.showSettingsChannels(c)
	case strings.HasPrefix(data, "settings:channels:view:"):
		return h.showSettingsChannelDetail(c, data)
	case data == "settings:channels:add":
		return h.promptAddChannel(c)
	case data == "settings:channels:confirm":
		return h.confirmAddChannel(c)
	case data == "settings:channels:cancel-link":
		return h.cancelAddChannel(c)
	case strings.HasPrefix(data, "settings:channels:delete:"):
		return h.deleteChannel(c, data)
	case data == "settings:google":
		return h.showGoogleSettings(c)
	case data == "settings:notifications":
		return h.showNotificationSettings(c)
	case data == "settings:notifications:toggle":
		return h.toggleNotifications(c)
	case data == "settings:notifications:time":
		return h.promptNotificationTime(c)
	case data == "settings:notifications:timezone":
		return h.promptTimezone(c)
	default:
		return respond(c, "<b>Неизвестное действие.</b>", nil)
	}
}

func (h *Handler) showCampaignsMenu(c tele.Context) error {
	menu := core_telegram_ui.InlineMenu(
		core_telegram_ui.Row(core_telegram_ui.Btn("➕ Создать", "campaigns:create")),
		core_telegram_ui.Row(
			core_telegram_ui.Btn("🟢 Активные", "campaigns:list:active:1"),
			core_telegram_ui.Btn("🔴 Закрытые", "campaigns:list:closed:1"),
		),
		core_telegram_ui.Row(
			core_telegram_ui.Btn("📚 Все", "campaigns:list:all:1"),
		),
	)
	return respond(c, "<b>📣 Рекламные кампании</b>\nВыберите действие ниже.", menu)
}

func (h *Handler) showSettingsMenu(c tele.Context) error {
	menu := core_telegram_ui.InlineMenu(
		core_telegram_ui.Row(core_telegram_ui.Btn("🔗 Каналы", "settings:channels")),
		core_telegram_ui.Row(core_telegram_ui.Btn("☁️ Google", "settings:google")),
		core_telegram_ui.Row(core_telegram_ui.Btn("🔔 Уведомления", "settings:notifications")),
	)
	return respond(c, "<b>⚙️ Настройки</b>\nВыберите раздел ниже.", menu)
}

func (h *Handler) showCampaignList(c tele.Context, data string) error {
	parts := strings.Split(data, ":")
	if len(parts) != 4 {
		return respond(c, "<b>Некорректная пагинация кампаний.</b>", nil)
	}
	status := parts[2]
	if status != domain.CampaignStatusActive && status != domain.CampaignStatusClosed && status != "all" {
		return h.showCampaignsMenu(c)
	}
	page, _ := strconv.Atoi(parts[3])
	list, err := h.client.ListCampaigns(context.Background(), c.Sender().ID, status, page, domain.DefaultCampaignsPageSize)
	if err != nil {
		return respond(c, "<b>Не удалось получить список кампаний.</b>", nil)
	}
	if len(list.Items) == 0 {
		return respond(c, "<b>Кампании не найдены.</b>", core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "campaigns:menu"))))
	}
	rows := make([][]core_telegram_ui.Button, 0, len(list.Items)+2)
	for _, item := range list.Items {
		rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn(campaignListLabel(item, status), formatCampaignViewCallback(item.ID, status, list.Page))))
	}
	if list.Total > len(list.Items) {
		pageRow := make([]core_telegram_ui.Button, 0, 3)
		if list.HasPrev {
			pageRow = append(pageRow, core_telegram_ui.Btn("←", fmt.Sprintf("campaigns:list:%s:%d", status, list.Page-1)))
		}
		pageRow = append(pageRow, core_telegram_ui.Btn(fmt.Sprintf("%d", list.Page), "campaigns:noop"))
		if list.HasNext {
			pageRow = append(pageRow, core_telegram_ui.Btn("→", fmt.Sprintf("campaigns:list:%s:%d", status, list.Page+1)))
		}
		rows = append(rows, pageRow)
	}
	rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "campaigns:menu")))
	return respond(c, fmt.Sprintf("<b>%s</b>", html.EscapeString(statusLabel(status))), core_telegram_ui.InlineMenu(rows...))
}

func (h *Handler) showCampaignDetail(c tele.Context, data string) error {
	id, origin, err := parseCampaignAction(data, "view")
	if err != nil {
		return respond(c, "<b>Некорректный идентификатор кампании.</b>", nil)
	}
	item, err := h.client.GetCampaign(context.Background(), c.Sender().ID, id)
	if err != nil {
		return respond(c, "<b>Не удалось получить кампанию.</b>", nil)
	}
	settings, _ := h.client.GetSettings(context.Background(), c.Sender().ID)
	return respond(c, formatCampaign(item), campaignDetailMenu(item, origin, settings))
}

func (h *Handler) closeCampaign(c tele.Context, data string) error {
	id, origin, err := parseCampaignAction(data, "close")
	if err != nil {
		return respond(c, "<b>Некорректный идентификатор кампании.</b>", nil)
	}
	item, err := h.client.CloseCampaign(context.Background(), c.Sender().ID, id)
	if err != nil {
		return respond(c, "<b>Не удалось закрыть кампанию.</b>", nil)
	}
	settings, _ := h.client.GetSettings(context.Background(), c.Sender().ID)
	return respond(c, "<b>Кампания закрыта досрочно.</b>\n\n"+formatCampaign(item), campaignDetailMenu(item, origin, settings))
}

func (h *Handler) refreshCampaign(c tele.Context, data string) error {
	id, origin, err := parseCampaignAction(data, "refresh")
	if err != nil {
		return respond(c, "<b>Некорректный идентификатор кампании.</b>", nil)
	}
	item, err := h.client.GetCampaign(context.Background(), c.Sender().ID, id)
	if err != nil {
		return respond(c, "<b>Не удалось получить кампанию.</b>", nil)
	}
	settings, _ := h.client.GetSettings(context.Background(), c.Sender().ID)
	if wait := refreshWait(item.LastSnapshotAt); wait > 0 {
		return respond(c, fmt.Sprintf("<b>Обновлять просмотры можно не чаще одного раза в час.</b>\nПопробуйте снова через <b>%s</b>.", html.EscapeString(formatDuration(wait))), campaignDetailMenu(item, origin, settings))
	}
	result, err := h.client.RefreshCampaign(context.Background(), c.Sender().ID, id)
	if err != nil {
		return respond(c, "<b>Не удалось обновить кампанию.</b>", campaignDetailMenu(item, origin, settings))
	}
	message := "Кампания обновлена."
	if result.AutoClosed {
		message = "Кампания автоматически закрыта по цели."
	}
	return respond(c, "<b>"+html.EscapeString(message)+"</b>\n\n"+formatCampaign(result.Campaign), campaignDetailMenu(result.Campaign, origin, settings))
}

func (h *Handler) createCampaignSpreadsheet(c tele.Context, data string) error {
	id, origin, err := parseCampaignAction(data, "sheet")
	if err != nil {
		return respond(c, "<b>Некорректный идентификатор кампании.</b>", nil)
	}
	settings, err := h.client.GetSettings(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить настройки пользователя.</b>", nil)
	}
	item, err := h.client.CreateCampaignSpreadsheet(context.Background(), c.Sender().ID, id)
	if err != nil {
		current, currentErr := h.client.GetCampaign(context.Background(), c.Sender().ID, id)
		if currentErr == nil {
			return respond(c, "<b>Не удалось создать Google Таблицу.</b>", campaignDetailMenu(current, origin, settings))
		}
		return respond(c, "<b>Не удалось создать Google Таблицу.</b>", nil)
	}
	return respond(c, "<b>✅ Google Таблица создана.</b>\n\n"+formatCampaign(item), campaignDetailMenu(item, origin, settings))
}

func (h *Handler) beginCampaignCreate(c tele.Context) error {
	channels, err := h.client.ListChannels(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить список каналов.</b>", nil)
	}
	if len(channels) == 0 {
		return respond(c, "<b>Сначала привяжите хотя бы один канал в настройках.</b>", core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("⚙️ К настройкам", "settings:channels"))))
	}
	if err := h.saveDraft(c.Sender().ID, stateCampaignKeyword, campaign_service.CreateCampaignDraft{Columns: domain.DefaultStatsColumns()}); err != nil {
		return respond(c, "<b>Не удалось открыть создание кампании.</b>", nil)
	}
	rows := make([][]core_telegram_ui.Button, 0, len(channels)+1)
	for _, channel := range channels {
		rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn(channelButtonLabel(channel), "campaigns:create:channel:"+channel.ChannelID)))
	}
	rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn("✖️ Отмена", "campaigns:create:cancel")))
	return respond(c, "<b>🆕 Новая рекламная кампания</b>\nВыберите канал для новой РК.", core_telegram_ui.InlineMenu(rows...))
}

func (h *Handler) selectCreateChannel(c tele.Context, data string) error {
	channelID := strings.TrimPrefix(data, "campaigns:create:channel:")
	draft, err := h.loadDraft(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить черновик кампании.</b>", nil)
	}
	draft.ChannelID = channelID
	if err := h.saveDraft(c.Sender().ID, stateCampaignManualTarget, draft); err != nil {
		return respond(c, "<b>Не удалось сохранить канал кампании.</b>", nil)
	}
	h.input.Set(c.Sender().ID, stateCampaignManualTarget)
	menu := core_telegram_ui.InlineMenu(
		core_telegram_ui.Row(core_telegram_ui.Btn("250K", "campaigns:create:target:250000"), core_telegram_ui.Btn("500K", "campaigns:create:target:500000")),
		core_telegram_ui.Row(core_telegram_ui.Btn("750K", "campaigns:create:target:750000"), core_telegram_ui.Btn("1 МЛН", "campaigns:create:target:1000000")),
		core_telegram_ui.Row(core_telegram_ui.Btn("1.5 МЛН", "campaigns:create:target:1500000"), core_telegram_ui.Btn("2 МЛН", "campaigns:create:target:2000000")),
		core_telegram_ui.Row(core_telegram_ui.Btn("3 МЛН", "campaigns:create:target:3000000"), core_telegram_ui.Btn("Не задано", "campaigns:create:target:none")),
		core_telegram_ui.Row(core_telegram_ui.Btn("✖️ Отмена", "campaigns:create:cancel")),
	)
	return respond(c, "<b>🎯 Шаг 2 из 5</b>\nВыберите цель по просмотрам или отправьте число сообщением.", menu)
}

func (h *Handler) selectCreateTarget(c tele.Context, data string) error {
	value := strings.TrimPrefix(data, "campaigns:create:target:")
	draft, err := h.loadDraft(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить черновик кампании.</b>", nil)
	}
	switch value {
	case "manual":
		if err := h.saveDraft(c.Sender().ID, stateCampaignManualTarget, draft); err != nil {
			return respond(c, "<b>Не удалось открыть ввод цели.</b>", nil)
		}
		h.input.Set(c.Sender().ID, stateCampaignManualTarget)
		return respond(c, "<b>Введите целевое количество просмотров.</b>\nНапример: <code>1250000</code>", nil)
	case "none":
		draft.TargetViews = nil
	default:
		number, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return respond(c, "<b>Некорректная цель кампании.</b>", nil)
		}
		draft.TargetViews = &number
	}
	if err := h.saveDraft(c.Sender().ID, stateCampaignKeyword, draft); err != nil {
		return respond(c, "<b>Не удалось сохранить цель кампании.</b>", nil)
	}
	return h.showCampaignColumnsEditor(c, draft)
}

func (h *Handler) handleCampaignManualTarget(c tele.Context) error {
	value, err := parseViews(strings.TrimSpace(c.Text()))
	if err != nil {
		return respond(c, "<b>Не удалось распознать число просмотров.</b>\nПопробуйте еще раз.", nil)
	}
	draft, err := h.loadDraft(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить черновик кампании.</b>", nil)
	}
	draft.TargetViews = &value
	if err := h.saveDraft(c.Sender().ID, stateCampaignKeyword, draft); err != nil {
		return respond(c, "<b>Не удалось сохранить цель кампании.</b>", nil)
	}
	return h.showCampaignColumnsEditor(c, draft)
}

func (h *Handler) handleCampaignColumnsText(c tele.Context) error {
	return respond(c, "<b>Используйте кнопки ниже, чтобы выбрать и переставить столбцы.</b>", nil)
}

func (h *Handler) showCampaignColumnsEditor(c tele.Context, draft campaign_service.CreateCampaignDraft) error {
	draft.Columns = normalizedDraftColumns(draft.Columns)
	if err := h.saveDraft(c.Sender().ID, stateCampaignColumns, draft); err != nil {
		return respond(c, "<b>Не удалось сохранить столбцы таблицы.</b>", nil)
	}
	h.input.Set(c.Sender().ID, stateCampaignColumns)
	rows := make([][]core_telegram_ui.Button, 0, len(draft.Columns)+8)
	for idx, column := range draft.Columns {
		buttons := []core_telegram_ui.Button{core_telegram_ui.Btn(statsColumnLabel(column), fmt.Sprintf("campaigns:create:columns:toggle:%s", column))}
		if idx > 0 {
			buttons = append(buttons, core_telegram_ui.Btn("⬆️", fmt.Sprintf("campaigns:create:columns:up:%s", column)))
		}
		if idx < len(draft.Columns)-1 {
			buttons = append(buttons, core_telegram_ui.Btn("⬇️", fmt.Sprintf("campaigns:create:columns:down:%s", column)))
		}
		rows = append(rows, core_telegram_ui.Row(buttons...))
	}
	for _, column := range allStatsColumns() {
		if containsStatsColumn(draft.Columns, column) {
			continue
		}
		rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn("➕ "+statsColumnLabel(column), fmt.Sprintf("campaigns:create:columns:toggle:%s", column))))
	}
	rows = append(rows,
		core_telegram_ui.Row(core_telegram_ui.Btn("✅ Продолжить", "campaigns:create:columns:done")),
		core_telegram_ui.Row(core_telegram_ui.Btn("✖️ Отмена", "campaigns:create:cancel")),
	)
	return respond(c, "<b>🧩 Шаг 3 из 5</b>\nВыберите столбцы для таблицы и настройте их порядок.", core_telegram_ui.InlineMenu(rows...))
}

func (h *Handler) handleCampaignColumnsAction(c tele.Context, data string) error {
	draft, err := h.loadDraft(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить черновик кампании.</b>", nil)
	}
	draft.Columns = normalizedDraftColumns(draft.Columns)
	action := strings.TrimPrefix(data, "campaigns:create:columns:")
	if action == "done" {
		if err := h.saveDraft(c.Sender().ID, stateCampaignKeyword, draft); err != nil {
			return respond(c, "<b>Не удалось сохранить столбцы таблицы.</b>", nil)
		}
		h.input.Set(c.Sender().ID, stateCampaignKeyword)
		return respond(c, "<b>✍️ Шаг 4 из 5</b>\nВведите ключевое слово рекламы.", nil)
	}
	parts := strings.SplitN(action, ":", 2)
	if len(parts) != 2 {
		return respond(c, "<b>Некорректное действие со столбцами.</b>", nil)
	}
	column := domain.StatsColumn(strings.TrimSpace(parts[1]))
	if !domain.IsValidStatsColumn(column) {
		return respond(c, "<b>Некорректный столбец.</b>", nil)
	}
	switch parts[0] {
	case "toggle":
		draft.Columns = toggleDraftColumn(draft.Columns, column)
	case "up":
		draft.Columns = moveDraftColumn(draft.Columns, column, -1)
	case "down":
		draft.Columns = moveDraftColumn(draft.Columns, column, 1)
	default:
		return respond(c, "<b>Некорректное действие со столбцами.</b>", nil)
	}
	return h.showCampaignColumnsEditor(c, draft)
}

func (h *Handler) handleCampaignKeyword(c tele.Context) error {
	keyword := c.Text()
	if strings.TrimSpace(keyword) == "" {
		return respond(c, "<b>Ключевое слово не может быть пустым.</b>", nil)
	}
	draft, err := h.loadDraft(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить черновик кампании.</b>", nil)
	}
	draft.Keyword = keyword
	if err := h.saveDraft(c.Sender().ID, stateCampaignStartDate, draft); err != nil {
		return respond(c, "<b>Не удалось сохранить ключевое слово кампании.</b>", nil)
	}
	h.input.Set(c.Sender().ID, stateCampaignStartDate)
	menu := core_telegram_ui.InlineMenu(
		core_telegram_ui.Row(core_telegram_ui.Btn("📅 Сегодняшняя дата", "campaigns:create:start:today")),
		core_telegram_ui.Row(core_telegram_ui.Btn("✖️ Отмена", "campaigns:create:cancel")),
	)
	return respond(c, "<b>📆 Шаг 5 из 5</b>\nВведите дату старта в одном из форматов:\n<code>ДД-ММ-ГГ</code>, <code>ДД.ММ.ГГГГ</code>, <code>ДД ММ ГГГГ</code>, <code>ДД ММ ГГ</code>, <code>ДД-ММ-ГГГГ</code>.", menu)
}

func (h *Handler) handleCampaignStartDate(c tele.Context) error {
	parsed, err := parseHumanDate(strings.TrimSpace(c.Text()))
	if err != nil {
		return respond(c, "<b>Не удалось распознать дату.</b>\nИспользуйте один из форматов: <code>ДД-ММ-ГГ</code>, <code>ДД.ММ.ГГГГ</code>, <code>ДД ММ ГГГГ</code>, <code>ДД ММ ГГ</code>, <code>ДД-ММ-ГГГГ</code>.", nil)
	}
	return h.finalizeCreateStartDate(c, parsed)
}

func (h *Handler) selectCreateStartDateToday(c tele.Context) error {
	today, err := h.currentUserDate(c)
	if err != nil {
		return respond(c, "<b>Не удалось определить текущую дату.</b>", nil)
	}
	return h.finalizeCreateStartDate(c, today)
}

func (h *Handler) finalizeCreateStartDate(c tele.Context, parsed time.Time) error {
	if err := h.ensureStartDateAllowed(c, parsed); err != nil {
		return err
	}
	draft, err := h.loadDraft(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить черновик кампании.</b>", nil)
	}
	draft.StartDate = parsed.Format("2006-01-02")
	if err := h.saveDraft(c.Sender().ID, stateCampaignStartDate, draft); err != nil {
		return respond(c, "<b>Не удалось сохранить дату старта.</b>", nil)
	}
	return respond(c, formatDraftPreview(draft), core_telegram_ui.InlineMenu(
		core_telegram_ui.Row(core_telegram_ui.Btn("✅ Создать", "campaigns:create:confirm"), core_telegram_ui.Btn("✖️ Отмена", "campaigns:create:cancel")),
	))
}

func (h *Handler) confirmCreateCampaign(c tele.Context) error {
	draft, err := h.loadDraft(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить черновик кампании.</b>", nil)
	}
	item, err := h.client.CreateCampaign(context.Background(), c.Sender().ID, draft.ChannelID, draft.Keyword, draft.StartDate, draft.TargetViews, normalizedDraftColumns(draft.Columns))
	if err != nil {
		return respond(c, "<b>Не удалось создать кампанию.</b>", nil)
	}
	_ = h.client.DeleteInputSession(context.Background(), c.Sender().ID)
	h.input.Clear(c.Sender().ID)
	return respond(c, "<b>✅ Кампания создана.</b>\n\n"+formatCampaign(item), core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("📣 К кампаниям", "campaigns:menu"))))
}

func (h *Handler) cancelCreateCampaign(c tele.Context) error {
	_ = h.client.DeleteInputSession(context.Background(), c.Sender().ID)
	h.input.Clear(c.Sender().ID)
	return respond(c, "<b>✖️ Создание кампании отменено.</b>", core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("📣 К кампаниям", "campaigns:menu"))))
}

func (h *Handler) showSettingsChannels(c tele.Context) error {
	channels, err := h.client.ListChannels(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить список каналов.</b>", nil)
	}
	rows := make([][]core_telegram_ui.Button, 0, len(channels)+2)
	rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn("🔗 Привязать канал", "settings:channels:add")))
	for _, channel := range channels {
		rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn(channelButtonLabel(channel), "settings:channels:view:"+channel.ChannelID)))
	}
	rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "settings:menu")))
	text := "<b>🔗 Привязанные каналы</b>"
	if len(channels) == 0 {
		text += "\nПока ничего не привязано."
	}
	return respond(c, text, core_telegram_ui.InlineMenu(rows...))
}

func (h *Handler) showSettingsChannelDetail(c tele.Context, data string) error {
	channelID := strings.TrimPrefix(data, "settings:channels:view:")
	channels, err := h.client.ListChannels(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить список каналов.</b>", nil)
	}
	for _, channel := range channels {
		if channel.ChannelID != channelID {
			continue
		}
		menu := core_telegram_ui.InlineMenu(
			core_telegram_ui.Row(core_telegram_ui.Btn("🗑️ Удалить", "settings:channels:delete:"+channel.ChannelID)),
			core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "settings:channels")),
		)
		return respond(c, formatChannelDetail(channel), menu)
	}
	return respond(c, "<b>Канал не найден.</b>", core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "settings:channels"))))
}

func (h *Handler) promptAddChannel(c tele.Context) error {
	if err := h.saveSession(c.Sender().ID, "settings", stateSettingsAddChannel, "{}"); err != nil {
		return respond(c, "<b>Не удалось открыть ввод ID канала.</b>", nil)
	}
	h.input.Set(c.Sender().ID, stateSettingsAddChannel)
	return respond(c, "<b>Отправьте ID канала, @handle или ссылку на канал.</b>\nНапример:\n<code>UCoV1F9JNNqripLok4BDLetQ</code>\n<code>@drake_afk</code>\n<code>https://www.youtube.com/channel/UCoV1F9JNNqripLok4BDLetQ/</code>", nil)
}

func (h *Handler) handleAddChannelText(c tele.Context) error {
	resolved, err := h.client.ResolveChannel(context.Background(), c.Sender().ID, strings.TrimSpace(c.Text()))
	if err != nil {
		return respond(c, "<b>Не удалось привязать канал.</b>", nil)
	}
	pending := pendingChannelVerification{
		ChannelID:        resolved.ChannelID,
		ChannelTitle:     resolved.ChannelTitle,
		VerificationCode: resolved.VerificationCode,
		OriginalInput:    strings.TrimSpace(c.Text()),
	}
	if err := h.savePendingChannelVerification(c.Sender().ID, pending); err != nil {
		return respond(c, "<b>Не удалось сохранить подтверждение канала.</b>", nil)
	}
	h.input.Set(c.Sender().ID, stateSettingsConfirmChannel)
	menu := core_telegram_ui.InlineMenu(
		core_telegram_ui.Row(core_telegram_ui.Btn("✅ Подтвердить", "settings:channels:confirm")),
		core_telegram_ui.Row(core_telegram_ui.Btn("✖️ Отмена", "settings:channels:cancel-link")),
	)
	return respond(c, formatChannelVerificationInstructions(pending, c.Sender().ID), menu)
}

func (h *Handler) confirmAddChannel(c tele.Context) error {
	pending, err := h.loadPendingChannelVerification(c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось восстановить подтверждение канала.</b>", core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("🔗 К каналам", "settings:channels"))))
	}
	channel, err := h.client.VerifyChannel(context.Background(), c.Sender().ID, pending.ChannelID)
	if err != nil {
		menu := core_telegram_ui.InlineMenu(
			core_telegram_ui.Row(core_telegram_ui.Btn("✅ Подтвердить", "settings:channels:confirm")),
			core_telegram_ui.Row(core_telegram_ui.Btn("✖️ Отмена", "settings:channels:cancel-link")),
		)
		return respond(c, "<b>Код подтверждения пока не найден в описании канала.</b>\nПроверьте, что код вставлен в описание и сохранен, затем нажмите подтверждение еще раз.\n\n"+formatChannelVerificationInstructions(pending, c.Sender().ID), menu)
	}
	_ = h.client.DeleteInputSession(context.Background(), c.Sender().ID)
	h.input.Clear(c.Sender().ID)
	return respond(c, "<b>✅ Канал привязан.</b>\nТеперь код можно удалить из описания канала.\n\n"+formatChannelDetail(channel), core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("🔗 К каналам", "settings:channels"))))
}

func (h *Handler) cancelAddChannel(c tele.Context) error {
	_ = h.client.DeleteInputSession(context.Background(), c.Sender().ID)
	h.input.Clear(c.Sender().ID)
	return h.showSettingsChannels(c)
}

func (h *Handler) deleteChannel(c tele.Context, data string) error {
	channelID := strings.TrimPrefix(data, "settings:channels:delete:")
	if err := h.client.DeleteChannel(context.Background(), c.Sender().ID, channelID); err != nil {
		return respond(c, "<b>Не удалось удалить канал.</b>", nil)
	}
	return h.showSettingsChannels(c)
}

func (h *Handler) showNotificationSettings(c tele.Context) error {
	settings, err := h.client.GetSettings(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить настройки уведомлений.</b>", nil)
	}
	text := fmt.Sprintf("<b>🔔 Уведомления</b>\nСтатус: <b>%s</b>\nВремя: <code>%s</code>\nТаймзона: <code>%s</code>", html.EscapeString(enabledLabel(settings.NotificationsEnabled)), html.EscapeString(settings.NotificationTime), html.EscapeString(settings.Timezone))
	menu := core_telegram_ui.InlineMenu(
		core_telegram_ui.Row(core_telegram_ui.Btn(toggleLabel(settings.NotificationsEnabled), "settings:notifications:toggle")),
		core_telegram_ui.Row(core_telegram_ui.Btn("🕒 Изменить время", "settings:notifications:time"), core_telegram_ui.Btn("🌍 Изменить таймзону", "settings:notifications:timezone")),
		core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "settings:menu")),
	)
	return respond(c, text, menu)
}

func (h *Handler) showGoogleSettings(c tele.Context) error {
	settings, err := h.client.GetSettings(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить настройки Google.</b>", nil)
	}
	text := "<b>☁️ Google</b>\nСтатус: <b>не подключен</b>"
	menu := core_telegram_ui.InlineMenu()
	linkURL, linkErr := h.client.GetGoogleLink(context.Background(), c.Sender().ID)
	if strings.TrimSpace(settings.GoogleEmail) != "" {
		text = "<b>☁️ Google</b>\nСтатус: <b>подключен</b>\nEmail: <code>" + html.EscapeString(settings.GoogleEmail) + "</code>"
	}
	if linkErr == nil && strings.TrimSpace(linkURL) != "" {
		connectLabel := "🔗 Подключить Google"
		if strings.TrimSpace(settings.GoogleEmail) != "" {
			connectLabel = "♻️ Переподключить Google"
		}
		menu = core_telegram_ui.InlineMenu(
			core_telegram_ui.Row(core_telegram_ui.Button{Text: connectLabel, URL: linkURL}),
			core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "settings:menu")),
		)
	} else {
		text += "\nGoogle интеграция сейчас недоступна."
		menu = core_telegram_ui.InlineMenu(core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", "settings:menu")))
	}
	return respond(c, text, menu)
}

func (h *Handler) toggleNotifications(c tele.Context) error {
	settings, err := h.client.GetSettings(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить настройки уведомлений.</b>", nil)
	}
	settings.NotificationsEnabled = !settings.NotificationsEnabled
	if _, err := h.client.UpdateSettings(context.Background(), settings); err != nil {
		return respond(c, "<b>Не удалось обновить уведомления.</b>", nil)
	}
	return h.showNotificationSettings(c)
}

func (h *Handler) promptNotificationTime(c tele.Context) error {
	if err := h.saveSession(c.Sender().ID, "settings", stateSettingsNotificationTime, "{}"); err != nil {
		return respond(c, "<b>Не удалось открыть ввод времени.</b>", nil)
	}
	h.input.Set(c.Sender().ID, stateSettingsNotificationTime)
	return respond(c, "<b>Введите время уведомлений в формате HH:MM.</b>", nil)
}

func (h *Handler) handleNotificationTimeText(c tele.Context) error {
	settings, err := h.client.GetSettings(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить текущие настройки.</b>", nil)
	}
	settings.NotificationTime = strings.TrimSpace(c.Text())
	if _, err := h.client.UpdateSettings(context.Background(), settings); err != nil {
		return respond(c, "<b>Не удалось обновить время уведомлений.</b>", nil)
	}
	_ = h.client.DeleteInputSession(context.Background(), c.Sender().ID)
	h.input.Clear(c.Sender().ID)
	return h.showNotificationSettings(c)
}

func (h *Handler) promptTimezone(c tele.Context) error {
	if err := h.saveSession(c.Sender().ID, "settings", stateSettingsTimezone, "{}"); err != nil {
		return respond(c, "<b>Не удалось открыть ввод таймзоны.</b>", nil)
	}
	h.input.Set(c.Sender().ID, stateSettingsTimezone)
	return respond(c, "<b>Введите таймзону.</b>\nНапример: <code>Europe/Moscow</code>", nil)
}

func (h *Handler) handleTimezoneText(c tele.Context) error {
	settings, err := h.client.GetSettings(context.Background(), c.Sender().ID)
	if err != nil {
		return respond(c, "<b>Не удалось получить текущие настройки.</b>", nil)
	}
	settings.Timezone = strings.TrimSpace(c.Text())
	if _, err := h.client.UpdateSettings(context.Background(), settings); err != nil {
		return respond(c, "<b>Не удалось обновить таймзону.</b>", nil)
	}
	_ = h.client.DeleteInputSession(context.Background(), c.Sender().ID)
	h.input.Clear(c.Sender().ID)
	return h.showNotificationSettings(c)
}

func (h *Handler) saveDraft(userID int64, step string, draft campaign_service.CreateCampaignDraft) error {
	payload, err := h.client.GetInputSession(context.Background(), userID)
	if err == nil {
		_ = payload
	}
	raw, err := jsonString(draft)
	if err != nil {
		return err
	}
	return h.saveSession(userID, flowCampaignCreate, step, raw)
}

func (h *Handler) saveSession(userID int64, flow string, step string, payload string) error {
	_, err := h.client.UpsertInputSession(context.Background(), domain.CampaignInputSession{
		TelegramUserID: userID,
		Flow:           flow,
		Step:           step,
		Payload:        payload,
	})
	return err
}

func (h *Handler) savePendingChannelVerification(userID int64, pending pendingChannelVerification) error {
	raw, err := jsonString(pending)
	if err != nil {
		return err
	}
	return h.saveSession(userID, "settings", stateSettingsConfirmChannel, raw)
}

func (h *Handler) loadDraft(userID int64) (campaign_service.CreateCampaignDraft, error) {
	session, err := h.client.GetInputSession(context.Background(), userID)
	if err != nil {
		return campaign_service.CreateCampaignDraft{}, err
	}
	return h.parseDraft(session.Payload)
}

func (h *Handler) loadPendingChannelVerification(userID int64) (pendingChannelVerification, error) {
	session, err := h.client.GetInputSession(context.Background(), userID)
	if err != nil {
		return pendingChannelVerification{}, err
	}
	var pending pendingChannelVerification
	if err := jsonUnmarshal(session.Payload, &pending); err != nil {
		return pendingChannelVerification{}, err
	}
	return pending, nil
}

func (h *Handler) parseDraft(payload string) (campaign_service.CreateCampaignDraft, error) {
	return h.clientDraftFromPayload(payload)
}

func (h *Handler) clientDraftFromPayload(payload string) (campaign_service.CreateCampaignDraft, error) {
	var draft campaign_service.CreateCampaignDraft
	if strings.TrimSpace(payload) == "" {
		return draft, nil
	}
	if err := jsonUnmarshal(payload, &draft); err != nil {
		return campaign_service.CreateCampaignDraft{}, err
	}
	return draft, nil
}

func formatCampaign(item campaignapi_client.Campaign) string {
	lines := []string{
		"<b>" + html.EscapeString(item.Title) + "</b>",
		fmt.Sprintf("Статус: %s <b>%s</b>", campaignStatusEmoji(item.Status), html.EscapeString(statusValueLabel(item.Status))),
	}
	if item.ChannelTitle != "" {
		lines = append(lines, "Канал: <b>"+html.EscapeString(item.ChannelTitle)+"</b>")
	}
	lines = append(lines,
		"ID канала: <code>"+html.EscapeString(item.ChannelID)+"</code>",
		"Ключевое слово: <code>"+html.EscapeString(item.Keyword)+"</code>",
		"Старт: <b>"+html.EscapeString(humanDate(item.StartDate))+"</b>",
		"Закрыто просмотров: <b>"+domain.FormatViewsNumber(item.LastTotalViews)+"</b>",
	)
	if item.TargetViews != nil {
		lines = append(lines, "Цель: <b>"+html.EscapeString(domain.FormatViewsTarget(*item.TargetViews))+"</b>")
		remaining := *item.TargetViews - item.LastTotalViews
		if remaining < 0 {
			remaining = 0
		}
		lines = append(lines, "Осталось: <b>"+domain.FormatViewsNumber(remaining)+"</b>")
	}
	if len(item.Columns) > 0 {
		lines = append(lines, "Столбцы: <b>"+html.EscapeString(strings.Join(statsColumnLabels(item.Columns), ", "))+"</b>")
	}
	if item.LastDailyGrowth != nil {
		lines = append(lines, "Темп: <b>"+domain.FormatViewsNumber(*item.LastDailyGrowth)+"/день</b>")
	}
	if item.LastSnapshotAt != nil {
		lines = append(lines, "Последнее обновление: <b>"+html.EscapeString(formatDateTime(*item.LastSnapshotAt, item.Timezone))+"</b>")
	}
	if item.EstimatedCloseDate != "" {
		lines = append(lines, "Оценка закрытия: <b>"+html.EscapeString(humanDate(item.EstimatedCloseDate))+"</b>")
	}
	if item.ClosedAt != nil {
		lines = append(lines, "Закрыта: <b>"+html.EscapeString(formatDateTime(*item.ClosedAt, item.Timezone))+"</b>")
	}
	if strings.TrimSpace(item.SpreadsheetURL) != "" {
		lines = append(lines,
			"",
			"<b>Google Таблица</b>",
			"<code>"+html.EscapeString(item.SpreadsheetURL)+"</code>",
		)
	}
	if formula := buildImportFormula(item.ExportURL); formula != "" {
		lines = append(lines,
			"",
			"<b>Формула для Google Sheets</b>",
			"<pre>"+html.EscapeString(formula)+"</pre>",
		)
	}
	return strings.Join(lines, "\n")
}

func formatDraftPreview(draft campaign_service.CreateCampaignDraft) string {
	parts := []string{
		"<b>Проверьте новую рекламную кампанию</b>",
		"ID канала: <code>" + html.EscapeString(draft.ChannelID) + "</code>",
		"Ключевое слово: <code>" + html.EscapeString(draft.Keyword) + "</code>",
		"Старт: <b>" + html.EscapeString(humanDate(draft.StartDate)) + "</b>",
	}
	if draft.TargetViews != nil {
		parts = append(parts, "Цель: <b>"+html.EscapeString(domain.FormatViewsTarget(*draft.TargetViews))+"</b>")
	} else {
		parts = append(parts, "Цель: <b>не задана</b>")
	}
	parts = append(parts, "Столбцы: <b>"+html.EscapeString(strings.Join(statsColumnLabels(normalizedDraftColumns(draft.Columns)), ", "))+"</b>")
	return strings.Join(parts, "\n")
}

func parseCallbackID(data string) (int64, error) {
	parts := strings.Split(data, ":")
	if len(parts) == 0 {
		return 0, fmt.Errorf("%w: invalid callback data", core_errors.ErrInvalidArgument)
	}
	return strconv.ParseInt(parts[len(parts)-1], 10, 64)
}

func parseCampaignAction(data string, action string) (int64, campaignListOrigin, error) {
	parts := strings.Split(strings.TrimSpace(data), ":")
	if len(parts) < 3 || parts[0] != "campaigns" || parts[1] != action {
		return 0, campaignListOrigin{}, fmt.Errorf("%w: invalid callback data", core_errors.ErrInvalidArgument)
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, campaignListOrigin{}, err
	}
	if len(parts) < 5 {
		return id, campaignListOrigin{}, nil
	}
	page, err := strconv.Atoi(parts[4])
	if err != nil {
		return 0, campaignListOrigin{}, err
	}
	return id, normalizeCampaignOrigin(parts[3], page), nil
}

func parseHumanDate(value string) (time.Time, error) {
	for _, layout := range []string{"02.01.06", "02.01.2006", "02-01-06", "02-01-2006", "02 01 06", "02 01 2006"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date")
}

func parseViews(value string) (int64, error) {
	cleaned := strings.ToUpper(strings.TrimSpace(value))
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(cleaned, "K"):
		multiplier = 1000
		cleaned = strings.TrimSuffix(cleaned, "K")
	case strings.HasSuffix(cleaned, "M"):
		multiplier = 1000000
		cleaned = strings.TrimSuffix(cleaned, "M")
	case strings.HasSuffix(cleaned, "МЛН"):
		multiplier = 1000000
		cleaned = strings.TrimSuffix(cleaned, "МЛН")
	}
	if strings.Contains(cleaned, ".") {
		parsed, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return 0, err
		}
		return int64(parsed * float64(multiplier)), nil
	}
	parsed, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed * multiplier, nil
}

func statsColumnLabel(column domain.StatsColumn) string {
	switch column {
	case domain.StatsColumnID:
		return "ID"
	case domain.StatsColumnPublishDate:
		return "Дата публикации"
	case domain.StatsColumnVideoURL:
		return "Ссылка"
	case domain.StatsColumnViews:
		return "Просмотры"
	case domain.StatsColumnAdTimings:
		return "Тайминги рекламы"
	case domain.StatsColumnViewsUpdatedAt:
		return "Дата обновления"
	default:
		return string(column)
	}
}

func allStatsColumns() []domain.StatsColumn {
	return []domain.StatsColumn{
		domain.StatsColumnID,
		domain.StatsColumnPublishDate,
		domain.StatsColumnVideoURL,
		domain.StatsColumnViews,
		domain.StatsColumnAdTimings,
		domain.StatsColumnViewsUpdatedAt,
	}
}

func normalizedDraftColumns(columns []domain.StatsColumn) []domain.StatsColumn {
	return domain.NormalizeStatsColumns(columns)
}

func containsStatsColumn(columns []domain.StatsColumn, expected domain.StatsColumn) bool {
	for _, column := range columns {
		if column == expected {
			return true
		}
	}
	return false
}

func toggleDraftColumn(columns []domain.StatsColumn, target domain.StatsColumn) []domain.StatsColumn {
	columns = normalizedDraftColumns(columns)
	result := make([]domain.StatsColumn, 0, len(columns))
	removed := false
	for _, column := range columns {
		if column == target {
			removed = true
			continue
		}
		result = append(result, column)
	}
	if removed {
		if len(result) == 0 {
			return columns
		}
		return result
	}
	return append(result, target)
}

func moveDraftColumn(columns []domain.StatsColumn, target domain.StatsColumn, delta int) []domain.StatsColumn {
	columns = append([]domain.StatsColumn(nil), normalizedDraftColumns(columns)...)
	index := -1
	for i, column := range columns {
		if column == target {
			index = i
			break
		}
	}
	if index < 0 {
		return columns
	}
	next := index + delta
	if next < 0 || next >= len(columns) {
		return columns
	}
	columns[index], columns[next] = columns[next], columns[index]
	return columns
}

func statsColumnLabels(columns []domain.StatsColumn) []string {
	result := make([]string, 0, len(columns))
	for _, column := range columns {
		result = append(result, statsColumnLabel(column))
	}
	return result
}

func statusLabel(value string) string {
	switch value {
	case domain.CampaignStatusActive:
		return "Активные кампании"
	case domain.CampaignStatusScheduled:
		return "Запланированные"
	case domain.CampaignStatusClosed:
		return "Закрытые кампании"
	case "all":
		return "Все кампании"
	default:
		return value
	}
}

func enabledLabel(value bool) string {
	if value {
		return "включены"
	}
	return "выключены"
}

func toggleLabel(enabled bool) string {
	if enabled {
		return "Выключить"
	}
	return "Включить"
}

func jsonString(value any) (string, error) {
	data, err := jsonMarshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func jsonUnmarshal(raw string, out any) error {
	return json.Unmarshal([]byte(raw), out)
}

type WebhookHandler struct {
	log           *logger.Logger
	bot           *tele.Bot
	peerServiceID string
	peerSecret    string
	nonces        *serviceauth.NonceStore
	processed     *serviceauth.NonceStore
}

func NewWebhookHandler(log *logger.Logger, bot *tele.Bot, peerServiceID string, peerSecret string) *WebhookHandler {
	if log == nil {
		log = logger.New(logger.WithComponent("campaign.telegram.webhook"))
	}
	return &WebhookHandler{
		log:           log,
		bot:           bot,
		peerServiceID: strings.TrimSpace(peerServiceID),
		peerSecret:    strings.TrimSpace(peerSecret),
		nonces:        serviceauth.NewNonceStore(5 * time.Minute),
		processed:     serviceauth.NewNonceStore(24 * time.Hour),
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := serviceauth.VerifyRequest(r, h.peerServiceID, h.peerSecret, h.nonces); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var event domain.BotWebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(event.EventID) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !h.processed.Use(event.EventID, time.Now().UTC()) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"duplicate":true}`))
		return
	}
	var sendErr error
	switch event.Type {
	case domain.BotWebhookEventNotificationDigest:
		sendErr = h.sendDigest(event.Digest)
	case domain.BotWebhookEventCampaignAutoClosed:
		sendErr = h.sendAutoClosed(event.Campaign)
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if sendErr != nil {
		h.processed.Forget(event.EventID)
		h.log.Error("failed to send webhook telegram message", logger.Err(sendErr))
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (h *WebhookHandler) sendDigest(digest *domain.NotificationDigest) error {
	if digest == nil || h.bot == nil {
		return nil
	}
	items := append([]domain.NotificationDigestCampaign(nil), digest.Campaigns...)
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	parts := []string{"<b>Ежедневное обновление по активным РК</b>"}
	for _, item := range items {
		lines := []string{"<b>" + html.EscapeString(item.Title) + "</b>", "Закрыто: <b>" + domain.FormatViewsNumber(item.TotalViews) + "</b>"}
		if item.RemainingViews != nil {
			lines = append(lines, "Осталось: <b>"+domain.FormatViewsNumber(*item.RemainingViews)+"</b>")
		}
		if item.DailyGrowth != nil {
			lines = append(lines, "Темп: <b>"+domain.FormatViewsNumber(*item.DailyGrowth)+"/день</b>")
		}
		if item.EstimatedCloseInDays != nil {
			lines = append(lines, fmt.Sprintf("ETA: <b>через %d дн.</b>", *item.EstimatedCloseInDays))
		} else if item.EstimatedCloseDate != nil {
			lines = append(lines, "ETA: <b>"+item.EstimatedCloseDate.Format("2006-01-02")+"</b>")
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	_, err := h.bot.Send(&tele.User{ID: digest.UserID}, strings.Join(parts, "\n\n"), tele.ModeHTML, tele.NoPreview)
	return err
}

func (h *WebhookHandler) sendAutoClosed(campaign *domain.Campaign) error {
	if campaign == nil || h.bot == nil {
		return nil
	}
	message := "<b>Кампания автоматически закрыта по достижении цели.</b>\n\n<b>" + html.EscapeString(campaign.Title()) + "</b>"
	_, err := h.bot.Send(&tele.User{ID: campaign.TelegramUserID}, message, tele.ModeHTML, tele.NoPreview)
	return err
}

func respond(c tele.Context, text string, menu *tele.ReplyMarkup) error {
	if menu != nil {
		return c.EditOrSend(text, menu, tele.ModeHTML, tele.NoPreview)
	}
	return c.EditOrSend(text, tele.ModeHTML, tele.NoPreview)
}

func campaignDetailMenu(item campaignapi_client.Campaign, origin campaignListOrigin, settings domain.UserSettings) *tele.ReplyMarkup {
	origin = withCampaignOriginFallback(origin, item.Status)
	rows := make([][]core_telegram_ui.Button, 0, 4)
	if strings.TrimSpace(settings.GoogleEmail) != "" {
		if strings.TrimSpace(item.SpreadsheetURL) != "" {
			rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Button{Text: "📂 Открыть таблицу", URL: item.SpreadsheetURL}))
		} else {
			rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn("📝 Создать таблицу", formatCampaignActionCallback("sheet", item.ID, origin))))
		}
	}
	if item.Status != domain.CampaignStatusClosed {
		rows = append(rows, core_telegram_ui.Row(
			core_telegram_ui.Btn("🔄 Обновить просмотры", formatCampaignActionCallback("refresh", item.ID, origin)),
			core_telegram_ui.Btn("🛑 Закрыть досрочно", formatCampaignActionCallback("close", item.ID, origin)),
		))
	}
	rows = append(rows, core_telegram_ui.Row(core_telegram_ui.Btn("⬅️ Назад", fmt.Sprintf("campaigns:list:%s:%d", origin.Status, origin.Page))))
	return core_telegram_ui.InlineMenu(rows...)
}

func formatCampaignViewCallback(campaignID int64, status string, page int) string {
	origin := normalizeCampaignOrigin(status, page)
	return fmt.Sprintf("campaigns:view:%d:%s:%d", campaignID, origin.Status, origin.Page)
}

func formatCampaignActionCallback(action string, campaignID int64, origin campaignListOrigin) string {
	origin = withCampaignOriginFallback(origin, "all")
	return fmt.Sprintf("campaigns:%s:%d:%s:%d", action, campaignID, origin.Status, origin.Page)
}

func normalizeCampaignOrigin(status string, page int) campaignListOrigin {
	status = strings.TrimSpace(status)
	if status != domain.CampaignStatusActive && status != domain.CampaignStatusClosed && status != "all" {
		status = "all"
	}
	if page <= 0 {
		page = 1
	}
	return campaignListOrigin{Status: status, Page: page}
}

func withCampaignOriginFallback(origin campaignListOrigin, fallbackStatus string) campaignListOrigin {
	if origin.Status == "" {
		origin = normalizeCampaignOrigin(fallbackStatus, 1)
	} else {
		origin = normalizeCampaignOrigin(origin.Status, origin.Page)
	}
	return origin
}

func campaignListLabel(item campaignapi_client.Campaign, status string) string {
	label := item.Title
	if status == "all" {
		label = campaignStatusEmoji(item.Status) + " " + label
	}
	return truncateLabel(label, 48)
}

func channelButtonLabel(channel domain.UserChannel) string {
	label := channel.ChannelTitle
	if strings.TrimSpace(label) == "" {
		label = channel.ChannelID
	}
	return truncateLabel(label, 48)
}

func formatChannelDetail(channel domain.UserChannel) string {
	parts := []string{"<b>Канал</b>"}
	if channel.ChannelTitle != "" {
		parts = append(parts, "Название: <b>"+html.EscapeString(channel.ChannelTitle)+"</b>")
	}
	parts = append(parts, "ID канала: <code>"+html.EscapeString(channel.ChannelID)+"</code>")
	return strings.Join(parts, "\n")
}

func formatChannelVerificationInstructions(pending pendingChannelVerification, userID int64) string {
	parts := []string{
		"<b>Подтверждение канала</b>",
	}
	if pending.ChannelTitle != "" {
		parts = append(parts, "Канал: <b>"+html.EscapeString(pending.ChannelTitle)+"</b>")
	}
	parts = append(parts,
		"ID канала: <code>"+html.EscapeString(pending.ChannelID)+"</code>",
		"",
		"1. Добавьте этот код в описание YouTube-канала:",
		"<pre>"+html.EscapeString(pending.VerificationCode)+"</pre>",
		"2. Сохраните описание канала.",
		"3. Нажмите <b>✅ Подтвердить</b>.",
		"",
		"После успешной привязки код можно удалить из описания.",
	)
	if userID != 0 {
		parts = append(parts, "Подтверждение привязано к вашему Telegram ID: <code>"+strconv.FormatInt(userID, 10)+"</code>")
	}
	return strings.Join(parts, "\n")
}

func campaignStatusEmoji(value string) string {
	switch value {
	case domain.CampaignStatusActive:
		return "🟢"
	case domain.CampaignStatusClosed:
		return "🔴"
	case domain.CampaignStatusScheduled:
		return "🟡"
	default:
		return "⚪"
	}
}

func statusValueLabel(value string) string {
	switch value {
	case domain.CampaignStatusActive:
		return "Активная"
	case domain.CampaignStatusClosed:
		return "Закрытая"
	case domain.CampaignStatusScheduled:
		return "Запланированная"
	default:
		return value
	}
}

func buildImportFormula(url string) string {
	url = strings.TrimSpace(url)
	if url == "" {
		return ""
	}
	return fmt.Sprintf("=IMPORTDATA(\"%s\";\",\";\"en_US\")", strings.ReplaceAll(url, "\"", "\"\""))
}

func humanDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.Format("02.01.2006")
	}
	return value
}

func formatDateTime(value time.Time, timezone string) string {
	loc := time.UTC
	if strings.TrimSpace(timezone) != "" {
		if loaded, err := time.LoadLocation(timezone); err == nil {
			loc = loaded
		}
	}
	return value.In(loc).Format("02.01.2006 15:04")
}

func truncateLabel(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
}

func refreshWait(lastSnapshotAt *time.Time) time.Duration {
	if lastSnapshotAt == nil {
		return 0
	}
	remaining := lastSnapshotAt.Add(time.Hour).Sub(time.Now().UTC())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func formatDuration(value time.Duration) string {
	if value <= 0 {
		return "0 мин"
	}
	minutes := int(value.Round(time.Minute) / time.Minute)
	if minutes <= 0 {
		minutes = 1
	}
	hours := minutes / 60
	minutes = minutes % 60
	if hours == 0 {
		return fmt.Sprintf("%d мин", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%d ч", hours)
	}
	return fmt.Sprintf("%d ч %d мин", hours, minutes)
}

func (h *Handler) currentUserDate(c tele.Context) (time.Time, error) {
	settings, err := h.client.GetSettings(context.Background(), c.Sender().ID)
	if err != nil {
		return time.Time{}, err
	}
	loc := mustLoadTelegramLocation(settings.Timezone)
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc), nil
}

func (h *Handler) ensureStartDateAllowed(c tele.Context, parsed time.Time) error {
	today, err := h.currentUserDate(c)
	if err != nil {
		return respond(c, "<b>Не удалось получить текущие настройки пользователя.</b>", nil)
	}
	candidate := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, today.Location())
	if candidate.After(today) {
		return respond(c, "<b>Дата старта не может быть позже сегодняшней.</b>", nil)
	}
	return nil
}

func mustLoadTelegramLocation(name string) *time.Location {
	if strings.TrimSpace(name) == "" {
		name = domain.DefaultUserTimezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
