package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validateInstance = validator.New()

func ValidateStruct(payload any) map[string]string {
	err := validateInstance.Struct(payload)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return map[string]string{"error": "invalid payload"}
	}

	errors := make(map[string]string)
	for _, fieldError := range validationErrors {
		fieldName := strings.ToLower(fieldError.Field())
		errors[fieldName] = formatErrorMessage(fieldError)
	}

	return errors
}

func formatErrorMessage(fieldError validator.FieldError) string {
	switch fieldError.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", strings.ToLower(fieldError.Field()))
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", strings.ToLower(fieldError.Field()), fieldError.Param())
	case "max":
		return fmt.Sprintf("%s must not exceed %s characters", strings.ToLower(fieldError.Field()), fieldError.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", strings.ToLower(fieldError.Field()), strings.ReplaceAll(fieldError.Param(), " ", ", "))
	default:
		return fmt.Sprintf("%s failed on '%s' rule", strings.ToLower(fieldError.Field()), fieldError.Tag())
	}
}
