package common

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"getarcane.dev/types/v3"
)

func TestToAPIErrorMapsClassifiedValidationDetails(t *testing.T) {
	var err error = &types.FieldError{Field: "name", Err: errors.New("Name is required")}
	err = Classify(ErrValidation, err)

	apiErr := ToAPIError(err)

	require.Equal(t, http.StatusBadRequest, apiErr.HTTPStatus())
	require.Equal(t, APIErrorCodeValidationError, apiErr.Code)
	require.Equal(t, "Name is required", apiErr.Message)
	require.Equal(t, map[string]any{"field": "name"}, apiErr.Details)
}

func TestToAPIErrorMapsClassifiedConflictDetails(t *testing.T) {
	apiErr := ToAPIError(Classify(ErrConflict, &types.FieldError{Field: "name", Err: errors.New("name already in use")}))

	require.Equal(t, http.StatusConflict, apiErr.HTTPStatus())
	require.Equal(t, APIErrorCodeConflict, apiErr.Code)
	require.Equal(t, map[string]any{"field": "name"}, apiErr.Details)

	require.Nil(t, ToAPIError(Classify(ErrConflict, errors.New("busy"))).Details)
}
