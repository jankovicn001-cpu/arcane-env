package usergroup

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"kit.getarcane.app/normalization"

	"getarcane.dev/arcane/v3/internal/common"
	"getarcane.dev/types/v3"
)

// ensureNameAvailable folds case in Go because SQLite's lower() only folds ASCII.
func ensureNameAvailable(tx *gorm.DB, name, exceptID string) error {
	var names []string
	if err := tx.Model(&Group{}).Where("id <> ?", exceptID).Pluck("name", &names).Error; err != nil {
		return fmt.Errorf("failed to check user group name: %w", err)
	}
	if slices.ContainsFunc(names, func(existing string) bool { return strings.EqualFold(existing, name) }) {
		return nameTakenError(name)
	}
	return nil
}

func nameTakenError(name string) error {
	return fieldError(common.ErrConflict, "name", fmt.Errorf("a user group named %q already exists", name))
}

func fieldError(kind error, field string, err error) error {
	return common.Classify(kind, &types.FieldError{Field: field, Err: err})
}

func normalizeText(field, value string, minLength, maxLength int) (string, error) {
	text := normalization.Text(value, normalization.TextOptions{Trim: true, NFC: true})
	if length := utf8.RuneCountInString(text); length < minLength || length > maxLength {
		return "", fieldError(common.ErrValidation, field, fmt.Errorf("%s must be between %d and %d characters", field, minLength, maxLength))
	}
	return text, nil
}

// normalizeLinks trims each value and drops repeats that the case-insensitive unique index would reject.
func normalizeLinks(links []types.UserGroupLink) ([]types.UserGroupLink, error) {
	out := make([]types.UserGroupLink, 0, len(links))
	for _, link := range links {
		if link.Kind != sourceOidc && link.Kind != sourceLdap {
			return nil, fieldError(common.ErrValidation, "links", fmt.Errorf("link kind %q must be oidc or ldap", link.Kind))
		}
		value := strings.TrimSpace(link.Value)
		if length := utf8.RuneCountInString(value); length < 1 || length > maxLinkValueLength {
			return nil, fieldError(common.ErrValidation, "links", fmt.Errorf("link values must be between 1 and %d characters", maxLinkValueLength))
		}
		if slices.ContainsFunc(out, func(existing types.UserGroupLink) bool {
			return existing.Kind == link.Kind && strings.EqualFold(existing.Value, value)
		}) {
			continue
		}
		out = append(out, types.UserGroupLink{Kind: link.Kind, Value: value})
	}
	return out, nil
}

func syncedSourceLabel(source string) string {
	switch source {
	case sourceOidc:
		return "SSO"
	case sourceLdap:
		return "LDAP"
	default:
		return source
	}
}

func toGrantDTO(r grantRow) types.UserGroupRoleAssignment {
	return types.UserGroupRoleAssignment{
		EnvironmentID:      r.EnvironmentID,
		EnvironmentGroupID: r.EnvironmentGroupID,
		ID:                 r.ID,
		RoleID:             r.RoleID,
		RoleName:           r.RoleName,
	}
}

func toAccessEntry(r grantRow) types.EnvironmentAccessEntry {
	return types.EnvironmentAccessEntry{
		ID:            r.ID,
		PrincipalType: types.EnvironmentAccessPrincipalGroup,
		UserGroupID:   r.UserGroupID,
		UserGroupName: r.UserGroupName,
		RoleID:        r.RoleID,
		RoleName:      r.RoleName,
		Source:        sourceManual,
	}
}
