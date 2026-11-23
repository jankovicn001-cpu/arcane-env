import type { OidcRoleMapping, Role } from '../role';
import type { CreateRequest, UpdateRequest } from './pagination';

export type RoleWritable = 'name' | 'description' | 'permissions';

export type RoleCreate = CreateRequest<Role, RoleWritable, 'name' | 'permissions'>;

export type RoleUpdate = UpdateRequest<Role, RoleWritable>;

export type OidcRoleMappingWritable = 'claimValue' | 'roleId' | 'environmentId' | 'environmentGroupId';

export type OidcRoleMappingCreate = CreateRequest<OidcRoleMapping, OidcRoleMappingWritable, 'claimValue' | 'roleId'>;

export type OidcRoleMappingUpdate = UpdateRequest<OidcRoleMapping, OidcRoleMappingWritable>;

/** Grants a role to exactly one user or one user group. */
export type EnvironmentAccessCreate = { roleId: string } & ({ userId: string } | { userGroupId: string });
