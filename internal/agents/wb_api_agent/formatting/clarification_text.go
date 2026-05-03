package formatting

import (
	"fmt"
	"strings"
)

// PURPOSE: Keeps user-facing clarification wording separate from validation and internal WB field names.
func MissingInputQuestion(inputName string) string {
	switch canonicalClarificationFieldName(inputName) {
	case "warehouse_id":
		return "Provide the seller warehouse ID or warehouse name."
	case "chrt_ids":
		return "Provide product identifiers: WB article IDs, seller vendor codes, or barcodes."
	case "date_from":
		return "Provide the period start date."
	case "date_to":
		return "Provide the period end date."
	default:
		return fmt.Sprintf("Provide %s.", humanizeFieldName(inputName))
	}
}

func MissingRequestBodyFieldQuestion(fieldName string) string {
	switch canonicalClarificationFieldName(fieldName) {
	case "warehouse_id":
		return "Provide the seller warehouse ID or warehouse name."
	case "chrt_ids":
		return "Provide product identifiers: WB article IDs, seller vendor codes, or barcodes."
	case "date_from":
		return "Provide the period start date."
	case "date_to":
		return "Provide the period end date."
	default:
		return fmt.Sprintf("Provide %s.", humanizeFieldName(fieldName))
	}
}

func MissingRequestBodyFieldsQuestion(fieldNames []string) string {
	questions := make([]string, 0, len(fieldNames))

	for _, fieldName := range fieldNames {
		// WHY: Multiple missing body fields must use the same bounded user-facing wording as single-field clarification.
		questions = append(questions, strings.TrimSuffix(MissingRequestBodyFieldQuestion(fieldName), "."))
	}

	return fmt.Sprintf("Provide required request details: %s.", strings.Join(questions, "; "))
}

func humanizeFieldName(fieldName string) string {
	normalized := strings.TrimSpace(fieldName)
	normalized = strings.TrimPrefix(normalized, "entities.")
	normalized = canonicalClarificationFieldName(normalized)
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.Join(strings.Fields(normalized), " ")

	switch normalized {
	case "":
		return "the required value"
	case "id":
		return "ID"
	case "nm id":
		return "WB article ID"
	case "imt id":
		return "grouped card ID"
	default:
		return normalized
	}
}

func canonicalClarificationFieldName(name string) string {
	switch name {
	case "warehouseId", "warehouseID", "warehouse_id":
		return "warehouse_id"
	case "chrtIds", "chrtIDs", "chrt_ids":
		return "chrt_ids"
	case "dateFrom", "date_from":
		return "date_from"
	case "dateTo", "date_to":
		return "date_to"
	default:
		return strings.ToLower(name)
	}
}
