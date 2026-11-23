import type { BadgeVariant } from '#lib/components/ui/badge/index.js';
import { m } from '#lib/paraglide/messages.js';
import type { RoleAssignmentSource } from '#lib/types/user.js';

/** Badge for where a grant or membership comes from: SSO and LDAP are synced, manual is set by an admin. */
export function assignmentSourceBadge(source: RoleAssignmentSource): { label: string; variant: BadgeVariant } {
	switch (source) {
		case 'oidc':
			return { label: m.sso(), variant: 'sky' };
		case 'ldap':
			return { label: m.ldap(), variant: 'blue' };
		default:
			return { label: m.manual(), variant: 'gray' };
	}
}
