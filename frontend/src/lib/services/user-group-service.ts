import type {
	UserGroupCreate,
	UserGroupMemberCreate,
	UserGroupRoleAssignmentCreate,
	UserGroupUpdate
} from '#lib/types/ui/usergroup.js';
import type { UserGroup, UserGroupMember, UserGroupRoleAssignment } from '#lib/types/usergroup.js';

import BaseAPIService from './api-service';

class UserGroupService extends BaseAPIService {
	async list(): Promise<UserGroup[]> {
		return this.handleResponse(this.api.get('/user-groups'));
	}

	async get(groupId: string): Promise<UserGroup> {
		return this.handleResponse(this.api.get(`/user-groups/${groupId}`));
	}

	async create(dto: UserGroupCreate): Promise<UserGroup> {
		return this.handleResponse(this.api.post('/user-groups', dto));
	}

	async update(groupId: string, dto: UserGroupUpdate): Promise<UserGroup> {
		return this.handleResponse(this.api.patch(`/user-groups/${groupId}`, dto));
	}

	async delete(groupId: string): Promise<void> {
		await this.handleResponse(this.api.delete(`/user-groups/${groupId}`));
	}

	async listMembers(groupId: string): Promise<UserGroupMember[]> {
		return this.handleResponse(this.api.get(`/user-groups/${groupId}/members`));
	}

	async addMember(groupId: string, dto: UserGroupMemberCreate): Promise<UserGroupMember> {
		return this.handleResponse(this.api.post(`/user-groups/${groupId}/members`, dto));
	}

	async removeMember(groupId: string, memberId: string): Promise<void> {
		await this.handleResponse(this.api.delete(`/user-groups/${groupId}/members/${memberId}`));
	}

	async listAssignments(groupId: string): Promise<UserGroupRoleAssignment[]> {
		return this.handleResponse(this.api.get(`/user-groups/${groupId}/role-assignments`));
	}

	async grant(groupId: string, dto: UserGroupRoleAssignmentCreate): Promise<UserGroupRoleAssignment> {
		return this.handleResponse(this.api.post(`/user-groups/${groupId}/role-assignments`, dto));
	}

	async revoke(groupId: string, assignmentId: string): Promise<void> {
		await this.handleResponse(this.api.delete(`/user-groups/${groupId}/role-assignments/${assignmentId}`));
	}
}

export const userGroupService = new UserGroupService();
