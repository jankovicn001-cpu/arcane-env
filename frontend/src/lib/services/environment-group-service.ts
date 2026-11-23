import type { EnvironmentGroup } from '#lib/types/environment.js';
import type { EnvironmentAccessEntry } from '#lib/types/role.js';
import type { EnvironmentGroupCreate, EnvironmentGroupUpdate } from '#lib/types/ui/environment.js';
import type { EnvironmentAccessCreate } from '#lib/types/ui/role.js';

import BaseAPIService from './api-service';

class EnvironmentGroupService extends BaseAPIService {
	async list(): Promise<EnvironmentGroup[]> {
		return this.handleResponse(this.api.get('/environment-groups'));
	}

	async create(dto: EnvironmentGroupCreate): Promise<EnvironmentGroup> {
		return this.handleResponse(this.api.post('/environment-groups', dto));
	}

	async update(groupId: string, dto: EnvironmentGroupUpdate): Promise<EnvironmentGroup> {
		return this.handleResponse(this.api.patch(`/environment-groups/${groupId}`, dto));
	}

	async delete(groupId: string): Promise<void> {
		await this.handleResponse(this.api.delete(`/environment-groups/${groupId}`));
	}

	async listAccess(groupId: string): Promise<EnvironmentAccessEntry[]> {
		return this.handleResponse(this.api.get(`/environment-groups/${groupId}/role-assignments`));
	}

	async grantAccess(groupId: string, dto: EnvironmentAccessCreate): Promise<EnvironmentAccessEntry> {
		return this.handleResponse(this.api.post(`/environment-groups/${groupId}/role-assignments`, dto));
	}

	async revokeAccess(groupId: string, assignmentId: string): Promise<void> {
		await this.handleResponse(this.api.delete(`/environment-groups/${groupId}/role-assignments/${assignmentId}`));
	}
}

export const environmentGroupService = new EnvironmentGroupService();
