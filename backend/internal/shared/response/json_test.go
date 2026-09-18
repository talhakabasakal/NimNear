package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/stretchr/testify/assert"
)

func TestError_ClientMessageSanitizedOnInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	err := domainErr.New(domainErr.ErrInternal, "database connection failed", errors.New("host=db.internal:5432"))

	Error(rec, err)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var body domainErr.ErrorResponse
	assert.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "an internal error occurred", body.Message)
	assert.NotContains(t, body.Message, "db.internal")
}

func TestError_IncludesNonSecretDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	err := domainErr.NewWithCodeAndDetails(
		domainErr.ErrBadRequest,
		"unsupported_nimiq_network",
		"Nimiq network does not match this deployment (requested_network=test-albatross requested_environment=testnet expected_network=main-albatross expected_environment=mainnet)",
		nil,
		map[string]string{
			"requested_network":     "test-albatross",
			"requested_environment": "testnet",
			"expected_network":      "main-albatross",
			"expected_environment":  "mainnet",
		},
	)

	Error(rec, err)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body domainErr.ErrorResponse
	assert.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "unsupported_nimiq_network", body.ErrorCode)
	assert.Equal(t, "test-albatross", body.Details["requested_network"])
	assert.Equal(t, "main-albatross", body.Details["expected_network"])
	assert.NotContains(t, rec.Body.String(), "signature")
	assert.NotContains(t, rec.Body.String(), "jwt")
}
