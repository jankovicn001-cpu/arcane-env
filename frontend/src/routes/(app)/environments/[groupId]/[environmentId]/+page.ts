import { resolve } from '$app/paths';
import { error, redirect } from '@sveltejs/kit';

import { m } from '#lib/paraglide/messages.js';
import { environmentStore } from '#lib/stores/environment.store.svelte.js';
import { isEnvironmentOnline } from '#lib/utils/docker.js';
import { browsableEnvironmentGroups } from '#lib/utils/environments.js';
import { tryCatch } from '#lib/utils/try-catch.js';

import type { PageLoad } from './$types';
import { environmentTab, preloadEnvironmentTab } from './environment-tabs';

export const load: PageLoad = async ({ params, parent, url }) => {
	const [{ queryClient }] = await Promise.all([parent(), environmentStore.ready]);
	const group = browsableEnvironmentGroups().find((candidate) =>
		candidate.environments.some((env) => env.id === params.environmentId)
	);
	const environment = group?.environments.find((env) => env.id === params.environmentId);
	if (!group || !environment) error(404, m.common_not_found_title({ resource: m.resource_environment_cap() }));
	if (group.id !== params.groupId) {
		redirect(
			307,
			`${resolve('/(app)/environments/[groupId]/[environmentId]', { groupId: group.id, environmentId: environment.id })}${url.search}`
		);
	}

	// Only the opened tab is preloaded; an offline or failing environment renders inline instead of failing the load.
	const live = environmentStore.available.find((env) => env.id === environment.id);
	const tab = environmentTab(environment.id, url.searchParams.get('tab'));
	if (tab && live && isEnvironmentOnline(live)) await tryCatch(preloadEnvironmentTab(queryClient, tab, environment));

	return { environment, group: { id: group.id, name: group.name } };
};
