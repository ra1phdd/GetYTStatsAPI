package campaign_google

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Account struct {
	Email        string
	RefreshToken string
}

type Spreadsheet struct {
	ID  string
	URL string
}

type Client struct {
	config *oauth2.Config
}

func New(clientID string, clientSecret string, redirectURL string) *Client {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	redirectURL = strings.TrimSpace(redirectURL)
	if clientID == "" || clientSecret == "" {
		return &Client{}
	}
	if redirectURL == "" {
		return &Client{}
	}
	return &Client{config: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			googleoauth2.UserinfoEmailScope,
			sheets.SpreadsheetsScope,
			drive.DriveFileScope,
		},
	}}
}

func (c *Client) Enabled() bool {
	return c != nil && c.config != nil
}

func (c *Client) AuthURL(state string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("google integration is not configured")
	}
	return c.config.AuthCodeURL(strings.TrimSpace(state), oauth2.AccessTypeOffline, oauth2.ApprovalForce), nil
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (Account, error) {
	if !c.Enabled() {
		return Account{}, fmt.Errorf("google integration is not configured")
	}
	token, err := c.config.Exchange(ctx, strings.TrimSpace(code))
	if err != nil {
		return Account{}, fmt.Errorf("exchange google code: %w", err)
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return Account{}, fmt.Errorf("google refresh token was not returned")
	}
	httpClient := c.config.Client(ctx, token)
	oauthService, err := googleoauth2.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return Account{}, fmt.Errorf("init google oauth service: %w", err)
	}
	userInfo, err := oauthService.Userinfo.Get().Do()
	if err != nil {
		return Account{}, fmt.Errorf("get google user info: %w", err)
	}
	return Account{Email: strings.TrimSpace(userInfo.Email), RefreshToken: strings.TrimSpace(token.RefreshToken)}, nil
}

func (c *Client) CreateSpreadsheet(ctx context.Context, refreshToken string, title string, formula string) (Spreadsheet, error) {
	if !c.Enabled() {
		return Spreadsheet{}, fmt.Errorf("google integration is not configured")
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return Spreadsheet{}, fmt.Errorf("google refresh token is empty")
	}
	tokenSource := c.config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	httpClient := oauth2.NewClient(ctx, tokenSource)
	sheetsService, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return Spreadsheet{}, fmt.Errorf("init google sheets service: %w", err)
	}
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return Spreadsheet{}, fmt.Errorf("init google drive service: %w", err)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "GetYTStats Campaign"
	}
	spreadsheet, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{Title: title},
	}).Do()
	if err != nil {
		return Spreadsheet{}, fmt.Errorf("create spreadsheet: %w", err)
	}
	if strings.TrimSpace(formula) != "" {
		_, err = sheetsService.Spreadsheets.Values.Update(spreadsheet.SpreadsheetId, "A1", &sheets.ValueRange{
			Values: [][]any{{formula}},
		}).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			return Spreadsheet{}, fmt.Errorf("write spreadsheet formula: %w", err)
		}
	}
	if err := c.shareSpreadsheetByLink(ctx, driveService, spreadsheet.SpreadsheetId); err != nil {
		return Spreadsheet{}, err
	}
	if err := c.resizeSpreadsheetColumns(ctx, sheetsService, spreadsheet); err != nil {
		return Spreadsheet{}, err
	}
	return Spreadsheet{ID: spreadsheet.SpreadsheetId, URL: spreadsheet.SpreadsheetUrl}, nil
}

func (c *Client) shareSpreadsheetByLink(ctx context.Context, driveService *drive.Service, spreadsheetID string) error {
	if driveService == nil || strings.TrimSpace(spreadsheetID) == "" {
		return nil
	}
	_, err := driveService.Permissions.Create(strings.TrimSpace(spreadsheetID), &drive.Permission{
		Type:               "anyone",
		Role:               "reader",
		AllowFileDiscovery: false,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("share spreadsheet by link: %w", err)
	}
	return nil
}

func (c *Client) resizeSpreadsheetColumns(ctx context.Context, sheetsService *sheets.Service, spreadsheet *sheets.Spreadsheet) error {
	if sheetsService == nil || spreadsheet == nil || strings.TrimSpace(spreadsheet.SpreadsheetId) == "" {
		return nil
	}
	var sheetID int64
	if len(spreadsheet.Sheets) > 0 && spreadsheet.Sheets[0] != nil && spreadsheet.Sheets[0].Properties != nil {
		sheetID = spreadsheet.Sheets[0].Properties.SheetId
	}
	requests := []*sheets.Request{
		{UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
			Range:      &sheets.DimensionRange{SheetId: sheetID, Dimension: "COLUMNS", StartIndex: 0, EndIndex: 1},
			Properties: &sheets.DimensionProperties{PixelSize: 160},
			Fields:     "pixelSize",
		}},
		{UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
			Range:      &sheets.DimensionRange{SheetId: sheetID, Dimension: "COLUMNS", StartIndex: 1, EndIndex: 2},
			Properties: &sheets.DimensionProperties{PixelSize: 140},
			Fields:     "pixelSize",
		}},
		{UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
			Range:      &sheets.DimensionRange{SheetId: sheetID, Dimension: "COLUMNS", StartIndex: 2, EndIndex: 3},
			Properties: &sheets.DimensionProperties{PixelSize: 360},
			Fields:     "pixelSize",
		}},
		{UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
			Range:      &sheets.DimensionRange{SheetId: sheetID, Dimension: "COLUMNS", StartIndex: 3, EndIndex: 4},
			Properties: &sheets.DimensionProperties{PixelSize: 120},
			Fields:     "pixelSize",
		}},
	}
	_, err := sheetsService.Spreadsheets.BatchUpdate(strings.TrimSpace(spreadsheet.SpreadsheetId), &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("resize spreadsheet columns: %w", err)
	}
	return nil
}
