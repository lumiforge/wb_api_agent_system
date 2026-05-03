package tools

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

const defaultBusinessTimezone = "Europe/Moscow"

type getCurrentDateTimeArgs struct {
	Timezone string `json:"timezone,omitempty"`
}

type getCurrentDateTimeResult struct {
	Timezone    string `json:"timezone"`
	CurrentDate string `json:"current_date"`
	CurrentTime string `json:"current_time"`
}

type resolveRelativePeriodArgs struct {
	PeriodKind  string `json:"period_kind"`
	CurrentDate string `json:"current_date,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
}

type resolveRelativePeriodResult struct {
	PeriodKind  string `json:"period_kind"`
	Timezone    string `json:"timezone"`
	CurrentDate string `json:"current_date"`
	DateFrom    string `json:"date_from"`
	DateTo      string `json:"date_to"`
}

func NewOperationSelectorTools() ([]tool.Tool, error) {
	currentDateTimeTool, err := functiontool.New(functiontool.Config{
		Name:        "get_current_datetime",
		Description: "Returns the current date and time for a requested IANA timezone. Use it when a user request contains relative dates or periods.",
	}, getCurrentDateTime)
	if err != nil {
		return nil, fmt.Errorf("create get_current_datetime tool: %w", err)
	}

	resolveRelativePeriodTool, err := functiontool.New(functiontool.Config{
		Name:        "resolve_relative_period",
		Description: "Resolves a semantic relative period kind into absolute YYYY-MM-DD dates using the current date in the requested timezone. period_kind must be one of: today, yesterday, last_7_days, last_30_days, current_week_to_date, previous_week, current_month_to_date, previous_month. current_date is optional and should only be used as an explicit YYYY-MM-DD override.",
	}, resolveRelativePeriod)
	if err != nil {
		return nil, fmt.Errorf("create resolve_relative_period tool: %w", err)
	}

	return []tool.Tool{
		currentDateTimeTool,
		resolveRelativePeriodTool,
	}, nil
}

func getCurrentDateTime(ctx tool.Context, args getCurrentDateTimeArgs) (getCurrentDateTimeResult, error) {
	timezone := args.Timezone
	if timezone == "" {
		timezone = defaultBusinessTimezone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return getCurrentDateTimeResult{}, fmt.Errorf("load timezone %q: %w", timezone, err)
	}

	now := time.Now().In(loc)

	return getCurrentDateTimeResult{
		Timezone:    timezone,
		CurrentDate: now.Format("2006-01-02"),
		CurrentTime: now.Format(time.RFC3339),
	}, nil
}

func resolveRelativePeriod(ctx tool.Context, args resolveRelativePeriodArgs) (resolveRelativePeriodResult, error) {
	timezone := args.Timezone
	if timezone == "" {
		timezone = defaultBusinessTimezone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return resolveRelativePeriodResult{}, fmt.Errorf("load timezone %q: %w", timezone, err)
	}

	currentDate := time.Now().In(loc)
	if strings.TrimSpace(args.CurrentDate) != "" {
		parsedCurrentDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(args.CurrentDate), loc)
		if err != nil {
			return resolveRelativePeriodResult{}, fmt.Errorf("parse current_date: %w", err)
		}
		currentDate = parsedCurrentDate
	}
	// WHY: current_date is a runtime fact owned by the tool, not a business input the LLM must provide.

	from, to, err := resolveRelativePeriodKind(args.PeriodKind, currentDate)
	if err != nil {
		return resolveRelativePeriodResult{}, err
	}
	y, m, d := currentDate.Date()
	normalizedCurrentDate := time.Date(y, m, d, 0, 0, 0, 0, loc)
	return resolveRelativePeriodResult{
		PeriodKind:  args.PeriodKind,
		Timezone:    timezone,
		CurrentDate: normalizedCurrentDate.Format("2006-01-02"),
		DateFrom:    from.Format("2006-01-02"),
		DateTo:      to.Format("2006-01-02"),
	}, nil
}

func resolveRelativePeriodKind(periodKind string, currentDate time.Time) (time.Time, time.Time, error) {
	y, m, d := currentDate.Date()
	loc := currentDate.Location()
	today := time.Date(y, m, d, 0, 0, 0, 0, loc)

	switch periodKind {
	case "today":
		return today, today, nil

	case "yesterday":
		yesterday := today.AddDate(0, 0, -1)
		return yesterday, yesterday, nil

	case "last_7_days":
		return today.AddDate(0, 0, -7), today, nil

	case "last_30_days":
		return today.AddDate(0, 0, -30), today, nil

	case "current_week_to_date":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return today.AddDate(0, 0, -(weekday - 1)), today, nil

	case "previous_week":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		currentWeekStart := today.AddDate(0, 0, -(weekday - 1))
		return currentWeekStart.AddDate(0, 0, -7), currentWeekStart.AddDate(0, 0, -1), nil

	case "current_month_to_date":
		return time.Date(y, m, 1, 0, 0, 0, 0, loc), today, nil

	case "previous_month":
		currentMonthStart := time.Date(y, m, 1, 0, 0, 0, 0, loc)
		previousMonthStart := currentMonthStart.AddDate(0, -1, 0)
		return previousMonthStart, currentMonthStart.AddDate(0, 0, -1), nil
	}

	return time.Time{}, time.Time{}, fmt.Errorf("unsupported period_kind %q", periodKind)
}
