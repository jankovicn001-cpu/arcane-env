import { environmentGroupListQuery } from '#lib/query/environment-groups.js';
import { environmentListQuery } from '#lib/query/environments.js';
import { roleAllQuery } from '#lib/query/roles.js';
import { userListQuery } from '#lib/query/users.js';
import type { EnvironmentGroup } from '#lib/types/environment.js';
import { GLOBAL_SCOPE } from '#lib/types/role.js';
import type { SearchPaginationSortRequest } from '#lib/types/ui/pagination.js';
import { hasPermission } from '#lib/utils/auth.js';
import { resolveInitialTableRequest } from '#lib/utils/tables.js';

import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { queryClient } = await parent();

	const userRequestOptions = resolveInitialTableRequest('arcane-users-table', {
		pagination: {
			page: 1,
			limit: 20
		},
		sort: {
			column: 'Username',
			direction: 'asc'
		}
	} satisfies SearchPaginationSortRequest);

	// Group names label environment-scoped assignments; only users who may list agents can load them.
	let groups: Promise<EnvironmentGroup[]> = Promise.resolve([]);
	if (hasPermission('environments:list', GLOBAL_SCOPE)) {
		groups = queryClient.query({ ...environmentGroupListQuery(), staleTime: 30_000 });
	}

	const [users, roles, environmentsPage, environmentGroups] = await Promise.all([
		queryClient.query(userListQuery(userRequestOptions)),
		queryClient.query({ ...roleAllQuery(), staleTime: 30_000 }),
		queryClient.query({ ...environmentListQuery({ pagination: { page: 1, limit: 1000 } }), staleTime: 30_000 }),
		groups
	]);

	return {
		users,
		userRequestOptions,
		roles,
		environments: environmentsPage.data,
		environmentGroups
	};
};
