import { queryOptions } from '@tanstack/svelte-query';

import { queryKeys } from '#lib/query/query-keys.js';
import { userGroupService } from '#lib/services/user-group-service.js';

export function userGroupListQuery() {
	return queryOptions({
		queryKey: queryKeys.userGroups.list(),
		queryFn: () => userGroupService.list()
	});
}

export function userGroupDetailQuery(groupId: string) {
	return queryOptions({
		queryKey: queryKeys.userGroups.detail(groupId),
		queryFn: () => userGroupService.get(groupId)
	});
}

export function userGroupMembersQuery(groupId: string) {
	return queryOptions({
		queryKey: queryKeys.userGroups.members(groupId),
		queryFn: () => userGroupService.listMembers(groupId)
	});
}

export function userGroupAssignmentsQuery(groupId: string) {
	return queryOptions({
		queryKey: queryKeys.userGroups.assignments(groupId),
		queryFn: () => userGroupService.listAssignments(groupId)
	});
}
