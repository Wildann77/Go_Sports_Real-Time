package security

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"sports-dashboard/internal/shared/enums"
)

// safe_slug checks if string is lowercase alphanumeric, dash, or underscore
var safeSlugRegex = regexp.MustCompile("^[a-z0-9-_]+$")

// RegisterCustomValidators registers all custom validation rules
func RegisterCustomValidators(v *validator.Validate) {
	v.RegisterValidation("non_empty_trimmed", validateNonEmptyTrimmed)
	v.RegisterValidation("match_status", validateMatchStatus)
	v.RegisterValidation("ws_event_type", validateWSEventType)
	v.RegisterValidation("safe_slug", validateSafeSlug)
	v.RegisterValidation("json_object", validateJSONObject)
}

func validateNonEmptyTrimmed(fl validator.FieldLevel) bool {
	return strings.TrimSpace(fl.Field().String()) != ""
}

func validateMatchStatus(fl validator.FieldLevel) bool {
	return enums.MatchStatus(fl.Field().String()).IsValid()
}

func validateWSEventType(fl validator.FieldLevel) bool {
	return enums.WSEventType(fl.Field().String()).IsValid()
}

func validateSafeSlug(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	return safeSlugRegex.MatchString(val)
}

func validateJSONObject(fl validator.FieldLevel) bool {
	val := fl.Field().Interface()
	if val == nil {
		return true // Allow nil or handle specifically if required
	}
	// Try to marshal back to JSON to check form
	bytes, err := json.Marshal(val)
	if err != nil {
		return false
	}
	var js map[string]interface{}
	err = json.Unmarshal(bytes, &js)
	return err == nil
}
