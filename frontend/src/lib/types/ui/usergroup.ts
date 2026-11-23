import type { UserGroup, UserGroupMember, UserGroupRoleAssignment } from '../usergroup';
import type { CreateRequest, UpdateRequest } from './pagination';

type UserGroupWritable = 'name' | 'description' | 'links';

export type UserGroupCreate = CreateRequest<UserGroup, UserGroupWritable, 'name'>;

export type UserGroupUpdate = UpdateRequest<UserGroup, UserGroupWritable>;

export type UserGroupMemberCreate = Pick<UserGroupMember, 'userId'>;

export type UserGroupRoleAssignmentCreate = CreateRequest<
	UserGroupRoleAssignment,
	'roleId' | 'environmentId' | 'environmentGroupId',
	'roleId'
>;
