package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Known Torn API error codes from the OpenAPI ApiError oneOf variants.
const (
	ErrorCodeUnknown                                  = 0
	ErrorCodeKeyEmpty                                 = 1
	ErrorCodeIncorrectKey                             = 2
	ErrorCodeWrongType                                = 3
	ErrorCodeWrongFields                              = 4
	ErrorCodeTooManyRequests                          = 5
	ErrorCodeIncorrectID                              = 6
	ErrorCodeIncorrectIDEntityRelation                = 7
	ErrorCodeIPBlocked                                = 8
	ErrorCodeAPIDisabled                              = 9
	ErrorCodeKeyOwnerInFederalJail                    = 10
	ErrorCodeKeyChangeCooldown                        = 11
	ErrorCodeKeyReadError                             = 12
	ErrorCodeKeyTemporaryDisabled                     = 13
	ErrorCodeDailyReadLimitReached                    = 14
	ErrorCodeLogUnavailable                           = 15
	ErrorCodeAccessLevelTooLow                        = 16
	ErrorCodeBackendError                             = 17
	ErrorCodeAPIKeyPaused                             = 18
	ErrorCodeMustMigrateToCrimesV2                    = 19
	ErrorCodeRaceNotFinished                          = 20
	ErrorCodeIncorrectCategory                        = 21
	ErrorCodeOnlyAvailableInAPIV1                     = 22
	ErrorCodeOnlyAvailableInAPIV2                     = 23
	ErrorCodeClosedTemporarily                        = 24
	ErrorCodeInvalidStatRequested                     = 25
	ErrorCodeOnlyCategoryOrStatsAllowed               = 26
	ErrorCodeMustMigrateToOrganizedCrimesV2           = 27
	ErrorCodeIncorrectLogID                           = 28
	ErrorCodeCategorySelectionUnavailableForLogs      = 29
	ErrorCodeFileDoesNotExist                         = 30
	ErrorCodeCityStatsCronFailed                      = 31
	ErrorCodeEndpointClosedUntilAttackingPeriod       = 32
	ErrorCodeNoCompetitionAvailable                   = 33
)

// APIError is a Torn API application-level error. Torn often returns these
// with HTTP 200 and a top-level {"error":{"code":N,"error":"..."}} body.
type APIError struct {
	Code       int
	Message    string
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("torn api error %d", e.Code)
	}
	return fmt.Sprintf("torn api error %d: %s", e.Code, e.Message)
}

// Temporary reports whether the error is worth retrying.
func (e *APIError) Temporary() bool {
	switch e.Code {
	case ErrorCodeTooManyRequests,
		ErrorCodeBackendError,
		ErrorCodeKeyTemporaryDisabled,
		ErrorCodeClosedTemporarily,
		ErrorCodeCityStatsCronFailed:
		return true
	default:
		return false
	}
}

// IsAPIError reports whether err is or wraps an *APIError.
func IsAPIError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr)
}

// AsAPIError extracts *APIError from err, if present.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

type tornErrorEnvelope struct {
	Error *struct {
		Code  int    `json:"code"`
		Error string `json:"error"`
	} `json:"error"`
}

// CheckBody inspects a Torn JSON response body for an application error.
// It returns nil when the body is not an error payload.
func CheckBody(body []byte) error {
	return checkBody(body, http.StatusOK)
}

func checkBody(body []byte, statusCode int) error {
	if len(body) == 0 {
		return nil
	}
	var envelope tornErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	if envelope.Error == nil {
		return nil
	}
	return &APIError{
		Code:       envelope.Error.Code,
		Message:    envelope.Error.Error,
		StatusCode: statusCode,
		Body:       append([]byte(nil), body...),
	}
}
