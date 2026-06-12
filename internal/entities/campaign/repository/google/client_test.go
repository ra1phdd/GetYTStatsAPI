package campaign_google

import (
	"testing"

	"getytstatsapi/internal/core/domain"
)

func TestBuildSpreadsheetFormatRequestsCentersFirstSixColumns(t *testing.T) {
	t.Parallel()

	requests := buildSpreadsheetFormatRequests(42, []domain.StatsColumn{
		domain.StatsColumnID,
		domain.StatsColumnPublishDate,
		domain.StatsColumnVideoURL,
		domain.StatsColumnViews,
		domain.StatsColumnAdTimings,
		domain.StatsColumnViewsUpdatedAt,
	})

	if len(requests) != 10 {
		t.Fatalf("len(requests) = %d, want 10", len(requests))
	}

	centerRequest := requests[6].RepeatCell
	if centerRequest == nil {
		t.Fatal("requests[6].RepeatCell = nil")
	}
	if centerRequest.Range == nil {
		t.Fatal("requests[6].RepeatCell.Range = nil")
	}
	if centerRequest.Range.SheetId != 42 {
		t.Fatalf("centerRequest.Range.SheetId = %d, want 42", centerRequest.Range.SheetId)
	}
	if centerRequest.Range.StartColumnIndex != 0 {
		t.Fatalf("centerRequest.Range.StartColumnIndex = %d, want 0", centerRequest.Range.StartColumnIndex)
	}
	if centerRequest.Range.EndColumnIndex != spreadsheetCenteredColumnsCount {
		t.Fatalf("centerRequest.Range.EndColumnIndex = %d, want %d", centerRequest.Range.EndColumnIndex, spreadsheetCenteredColumnsCount)
	}
	if centerRequest.Range.StartRowIndex != 0 || centerRequest.Range.EndRowIndex != 0 {
		t.Fatalf("centerRequest row bounds = (%d, %d), want (0, 0) for full-column formatting", centerRequest.Range.StartRowIndex, centerRequest.Range.EndRowIndex)
	}
	if centerRequest.Cell == nil || centerRequest.Cell.UserEnteredFormat == nil {
		t.Fatal("requests[6].RepeatCell.Cell.UserEnteredFormat = nil")
	}
	if centerRequest.Cell.UserEnteredFormat.HorizontalAlignment != "CENTER" {
		t.Fatalf("centerRequest.HorizontalAlignment = %q, want CENTER", centerRequest.Cell.UserEnteredFormat.HorizontalAlignment)
	}

	videoURLRequest := requests[7].RepeatCell
	if videoURLRequest == nil {
		t.Fatal("requests[7].RepeatCell = nil")
	}
	if videoURLRequest.Range == nil {
		t.Fatal("requests[7].RepeatCell.Range = nil")
	}
	if videoURLRequest.Range.StartColumnIndex != 2 || videoURLRequest.Range.EndColumnIndex != 3 {
		t.Fatalf("videoURLRequest column bounds = (%d, %d), want (2, 3)", videoURLRequest.Range.StartColumnIndex, videoURLRequest.Range.EndColumnIndex)
	}
	if videoURLRequest.Range.StartRowIndex != 1 || videoURLRequest.Range.EndRowIndex != 0 {
		t.Fatalf("videoURLRequest row bounds = (%d, %d), want (1, 0) for data rows only", videoURLRequest.Range.StartRowIndex, videoURLRequest.Range.EndRowIndex)
	}
	if videoURLRequest.Cell == nil || videoURLRequest.Cell.UserEnteredFormat == nil {
		t.Fatal("requests[7].RepeatCell.Cell.UserEnteredFormat = nil")
	}
	if videoURLRequest.Cell.UserEnteredFormat.HorizontalAlignment != "LEFT" {
		t.Fatalf("videoURLRequest.HorizontalAlignment = %q, want LEFT", videoURLRequest.Cell.UserEnteredFormat.HorizontalAlignment)
	}
}
