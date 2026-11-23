import type { RoleAssignmentSource } from './user';

export type UserGroupLinkKind = 'oidc' | 'ldap';

/** An SSO claim value or LDAP group name whose users join the group on login or sync. */
export type UserGroupLink = {
	kind: UserGroupLinkKind;
	value: string;
};

export type UserGroup = {
	id: string;
	name: string;
	description: string;
	memberCount: number;
	links: UserGroupLink[];
	createdAt: string;
	updatedAt: string;
};

export type UserGroupRef = Pick<UserGroup, 'id' | 'name'>;

export type UserGroupMember = {
	id: string;
	userId: string;
	username: string;
	displayName?: string;
	source: RoleAssignmentSource;
};

export type UserGroupRoleAssignment = {
	id: string;
	roleId: string;
	roleName: string;
	environmentId?: string;
	environmentGroupId?: string;
};
