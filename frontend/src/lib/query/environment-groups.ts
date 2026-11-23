import { queryOptions } from '@tanstack/svelte-query';

import { queryKeys } from '#lib/query/query-keys.js';
import { environmentGroupService } from '#lib/services/environment-group-service.js';

export function environmentGroupListQuery() {
	return queryOptions({
		queryKey: queryKeys.environmentGroups.list(),
		queryFn: () => environmentGroupService.list()
	});
}

export function environmentGroupAccessQuery(groupId: string) {
	return queryOptions({
		queryKey: queryKeys.environmentGroups.access(groupId),
		queryFn: () => environmentGroupService.listAccess(groupId)
	});
}
