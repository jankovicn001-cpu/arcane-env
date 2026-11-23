import { error, redirect } from '@sveltejs/kit';

import { m } from '#lib/paraglide/messages.js';
import { environmentGroupListQuery } from '#lib/query/environment-groups.js';
import { environmentListQuery } from '#lib/query/environments.js';
import { roleAllQuery } from '#lib/query/roles.js';
import { userGroupDetailQuery } from '#lib/query/user-groups.js';
import { userIsGlobalAdmin } from '#lib/utils/auth.js';
import { tryCatch } from '#lib/utils/try-catch.js';

import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent, params }) => {
	const { queryClient, user } = await parent();

	// Members and access are managed here, which only global admins may do.
	if (!userIsGlobalAdmin(user)) {
		throw redirect(302, '/system/user-groups');
	}

	const [group, roles, environmentsPage, environmentGroups] = await Promise.all([
		tryCatch(queryClient.query(userGroupDetailQuery(params.id))).then((result) => (result.error ? null : result.data)),
		queryClient.query({ ...roleAllQuery(), staleTime: 30_000 }),
		queryClient.query({ ...environmentListQuery({ pagination: { page: 1, limit: 1000 } }), staleTime: 30_000 }),
		queryClient.query({ ...environmentGroupListQuery(), staleTime: 30_000 })
	]);

	if (!group) {
		throw error(404, m.common_not_found_title({ resource: m.user_group() }));
	}

	return {
		group,
		roles,
		environments: environmentsPage.data,
		environmentGroups
	};
};
