package entities

import (
	"errors"
	"strings"
	"testing"
)

// PURPOSE: Protects the deterministic boundary before selector input reaches any LLM-based layer.
func TestNewOperationSelectionInputBuildsValidContract(t *testing.T) {
	readonly := true
	request := BusinessRequest{
		RequestID:              "request-1",
		Marketplace:            "wildberries",
		NaturalLanguageRequest: "Покажи товары",
		Constraints: BusinessConstraints{
			ReadonlyOnly:      true,
			NoJamSubscription: true,
		},
	}

	input := NewOperationSelectionInput(request, []WBRegistryOperation{
		{
			OperationID:           "operation-1",
			SourceFile:            "products.yaml",
			Method:                "POST",
			ServerURL:             "https://content-api.wildberries.ru",
			PathTemplate:          "/content/v2/get/cards/list",
			Tags:                  []string{"Карточки товаров"},
			Summary:               "Список карточек товаров",
			Description:           "Метод возвращает список созданных карточек товаров.",
			XReadonlyMethod:       &readonly,
			XTokenTypes:           []string{},
			RequestBodySchemaJSON: "{}",
			ResponseSchemaJSON:    "{}",
		},
	})

	if err := input.ValidateShape(); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if !input.Policies.NoSecrets {
		t.Fatalf("expected no_secrets policy to be true")
	}
	if !input.Policies.NoHTTPExecution {
		t.Fatalf("expected no_http_execution policy to be true")
	}
	if !input.Policies.RegistryOnly {
		t.Fatalf("expected registry_only policy to be true")
	}
	if input.RegistryCandidates[0].RiskLevel != "read" {
		t.Fatalf("expected read risk level, got %q", input.RegistryCandidates[0].RiskLevel)
	}
}

func TestOperationSelectionInputValidateShapeRejectsMissingCandidates(t *testing.T) {
	input := validOperationSelectionInput()
	input.RegistryCandidates = []OperationSelectionCandidate{}

	err := input.ValidateShape()

	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates must contain at least one candidate")
}

func TestOperationSelectionInputValidateShapeRejectsNilCandidates(t *testing.T) {
	input := validOperationSelectionInput()
	input.RegistryCandidates = nil

	err := input.ValidateShape()

	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates must be an array")
}

func TestOperationSelectionInputValidateShapeRequiresSafetyPolicies(t *testing.T) {
	input := validOperationSelectionInput()
	input.Policies.NoSecrets = false
	input.Policies.NoHTTPExecution = false
	input.Policies.RegistryOnly = false

	err := input.ValidateShape()

	assertOperationSelectionInputShapeErrorContains(t, err, "policies.no_secrets must be true")
	assertOperationSelectionInputShapeErrorContains(t, err, "policies.no_http_execution must be true")
	assertOperationSelectionInputShapeErrorContains(t, err, "policies.registry_only must be true")
}

func TestOperationSelectionInputValidateShapeRejectsInvalidCandidate(t *testing.T) {
	input := validOperationSelectionInput()
	input.RegistryCandidates[0].OperationID = ""
	input.RegistryCandidates[0].SourceFile = ""
	input.RegistryCandidates[0].Method = ""
	input.RegistryCandidates[0].ServerURL = ""
	input.RegistryCandidates[0].PathTemplate = ""
	input.RegistryCandidates[0].Tags = nil
	input.RegistryCandidates[0].XTokenTypes = nil

	err := input.ValidateShape()

	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates[0].operation_id is empty")
	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates[0].source_file is empty")
	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates[0].method is empty")
	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates[0].server_url is empty")
	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates[0].path_template is empty")
	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates[0].tags must be an array")
	assertOperationSelectionInputShapeErrorContains(t, err, "registry_candidates[0].x_token_types must be an array")
}

func TestOperationSelectionCandidateFromRegistryNormalizesNilSlices(t *testing.T) {
	candidate := OperationSelectionCandidateFromRegistry(WBRegistryOperation{
		OperationID:  "operation-1",
		SourceFile:   "products.yaml",
		Method:       "POST",
		ServerURL:    "https://content-api.wildberries.ru",
		PathTemplate: "/content/v2/get/cards/list",
		Tags:         nil,
		XTokenTypes:  nil,
	})

	if candidate.Tags == nil {
		t.Fatalf("expected tags to be non-nil")
	}
	if candidate.XTokenTypes == nil {
		t.Fatalf("expected x_token_types to be non-nil")
	}
}

func TestOperationSelectionCandidateFromRegistryMarksUnknownReadonlyRisk(t *testing.T) {
	candidate := OperationSelectionCandidateFromRegistry(WBRegistryOperation{
		OperationID:  "operation-1",
		SourceFile:   "products.yaml",
		Method:       "POST",
		ServerURL:    "https://content-api.wildberries.ru",
		PathTemplate: "/content/v2/get/cards/list",
		Tags:         []string{},
		XTokenTypes:  []string{},
	})

	if candidate.RiskLevel != "unknown" {
		t.Fatalf("expected unknown risk level, got %q", candidate.RiskLevel)
	}
}

func validOperationSelectionInput() OperationSelectionInput {
	readonly := true

	return OperationSelectionInput{
		SchemaVersion: "1.0",
		RequestID:     "request-1",
		Marketplace:   "wildberries",
		BusinessRequest: BusinessRequest{
			RequestID:              "request-1",
			Marketplace:            "wildberries",
			NaturalLanguageRequest: "Покажи товары",
		},
		RegistryCandidates: []OperationSelectionCandidate{
			{
				OperationID:        "operation-1",
				SourceFile:         "products.yaml",
				Method:             "POST",
				ServerURL:          "https://content-api.wildberries.ru",
				PathTemplate:       "/content/v2/get/cards/list",
				Tags:               []string{},
				Summary:            "Список карточек товаров",
				Description:        "Метод возвращает список созданных карточек товаров.",
				Readonly:           &readonly,
				RiskLevel:          "read",
				XTokenTypes:        []string{},
				RequiresJam:        false,
				XCategory:          "content",
				ResponseSchemaJSON: "{}",
			},
		},
		Policies: OperationSelectionPolicies{
			NoSecrets:       true,
			NoHTTPExecution: true,
			RegistryOnly:    true,
		},
	}
}

func assertOperationSelectionInputShapeErrorContains(t *testing.T, err error, expected string) {
	t.Helper()

	var shapeError OperationSelectionInputShapeValidationError
	if !errors.As(err, &shapeError) {
		t.Fatalf("expected OperationSelectionInputShapeValidationError, got %T: %v", err, err)
	}

	for _, message := range shapeError.Errors {
		if strings.Contains(message, expected) {
			return
		}
	}

	t.Fatalf("expected shape error containing %q, got %v", expected, shapeError.Errors)
}
