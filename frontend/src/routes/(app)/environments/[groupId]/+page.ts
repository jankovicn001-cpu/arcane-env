import { error } from '@sveltejs/kit';

import { m } from '#lib/paraglide/messages.js';
import { environmentGroupListQuery } from '#lib/query/environment-groups.js';
import { environmentStore } from '#lib/stores/environment.store.svelte.js';
import type { EnvironmentGroup } from '#lib/types/environment.js';
import { GLOBAL_SCOPE } from '#lib/types/role.js';
import { hasPermission } from '#lib/utils/auth.js';
import { browsableEnvironmentGroups } from '#lib/utils/environments.js';
import { tryCatch } from '#lib/utils/try-catch.js';

import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, parent }) => {
	const [{ queryClient }] = await Promise.all([parent(), environmentStore.ready]);
	// Admins can also open groups that have no agents yet.
	let managed: EnvironmentGroup[] | undefined;
	if (hasPermission('environments:list', GLOBAL_SCOPE)) {
		managed = (await tryCatch(queryClient.ensureQueryData(environmentGroupListQuery()))).data ?? undefined;
	}
	const group = browsableEnvironmentGroups(managed).find((candidate) => candidate.id === params.groupId);
	if (!group) error(404, m.common_not_found_title({ resource: m.environment() }));
	return { group };
};
