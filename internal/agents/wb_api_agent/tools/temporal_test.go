package tools

import (
	"testing"

	"google.golang.org/adk/tool"
)

func TestResolveRelativePeriodUsesCurrentDateOverride(t *testing.T) {
	got, err := resolveRelativePeriod(tool.Context(nil), resolveRelativePeriodArgs{
		PeriodKind:  "previous_week",
		CurrentDate: "2026-05-03",
		Timezone:    "Europe/Moscow",
	})
	if err != nil {
		t.Fatalf("resolve relative period: %v", err)
	}

	if got.CurrentDate != "2026-05-03" {
		t.Fatalf("unexpected current_date: %s", got.CurrentDate)
	}
	if got.DateFrom != "2026-04-20" {
		t.Fatalf("unexpected date_from: %s", got.DateFrom)
	}
	if got.DateTo != "2026-04-26" {
		t.Fatalf("unexpected date_to: %s", got.DateTo)
	}
}

func TestResolveRelativePeriodDoesNotRequireCurrentDate(t *testing.T) {
	got, err := resolveRelativePeriod(tool.Context(nil), resolveRelativePeriodArgs{
		PeriodKind: "previous_week",
		Timezone:   "Europe/Moscow",
	})
	if err != nil {
		t.Fatalf("resolve relative period without current_date: %v", err)
	}

	if got.CurrentDate == "" {
		t.Fatalf("current_date must be returned")
	}
	if got.DateFrom == "" {
		t.Fatalf("date_from must be returned")
	}
	if got.DateTo == "" {
		t.Fatalf("date_to must be returned")
	}
}

func TestResolveRelativePeriodKeepsInvalidCurrentDateErrorWhenOverrideProvided(t *testing.T) {
	_, err := resolveRelativePeriod(tool.Context(nil), resolveRelativePeriodArgs{
		PeriodKind:  "previous_week",
		CurrentDate: "03-05-2026",
		Timezone:    "Europe/Moscow",
	})
	if err == nil {
		t.Fatalf("expected invalid current_date error")
	}
}
