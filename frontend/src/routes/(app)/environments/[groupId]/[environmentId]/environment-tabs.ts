import type { QueryClient } from '@tanstack/svelte-query';

import type { TabItem } from '#lib/components/tab-bar/index.js';
import { ContainersIcon, ImagesIcon, NetworksIcon, ProjectsIcon, VolumesIcon } from '#lib/icons/index.js';
import { m } from '#lib/paraglide/messages.js';
import { containerListQuery } from '#lib/query/containers.js';
import { imageListQuery, imageUsageCountsQuery } from '#lib/query/images.js';
import { networkListQuery } from '#lib/query/networks.js';
import { projectListQuery, projectStatusCountsQuery, projectTagsQuery } from '#lib/query/projects.js';
import { volumeListQuery } from '#lib/query/volumes.js';
import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
import { featureStore } from '#lib/stores/features.store.svelte.js';
import type { SearchPaginationSortRequest } from '#lib/types/ui/pagination.js';
import { hasPermission } from '#lib/utils/auth.js';
import { ENVIRONMENT_RESOURCE_PERMISSIONS, type EnvironmentResource } from '#lib/utils/environments.js';
import { resolveInitialListPageRequest, resolveInitialTableRequest } from '#lib/utils/tables.js';

import { withArchivedFilter } from '../../../projects/projects-table.helpers';

/** Live reachability of the environment the page browses. */
export type EnvironmentReachability = { online: boolean; lastSeen?: string };

export const DEFAULT_ENVIRONMENT_TAB: EnvironmentResource = 'containers';

// Own keys, so environment filters saved by the cross-environment tables never apply here.
export const ENVIRONMENT_TABLE_KEYS: Record<EnvironmentResource, string> = {
	containers: 'arcane-environment-container-table',
	projects: 'arcane-environment-project-table',
	images: 'arcane-environment-image-table',
	volumes: 'arcane-environment-volume-table',
	networks: 'arcane-environment-network-table'
};

export function environmentTabItems(environmentId: string): (TabItem & { value: EnvironmentResource; visible: boolean })[] {
	const items: (TabItem & { value: EnvironmentResource })[] = [
		{ value: 'containers', label: m.containers(), icon: ContainersIcon },
		{ value: 'projects', label: m.projects_title(), icon: ProjectsIcon },
		{ value: 'images', label: m.images(), icon: ImagesIcon },
		{ value: 'volumes', label: m.resource_volumes_cap(), icon: VolumesIcon },
		{ value: 'networks', label: m.resource_networks_cap(), icon: NetworksIcon }
	];
	return items.map((item) => ({ ...item, visible: hasPermission(ENVIRONMENT_RESOURCE_PERMISSIONS[item.value], environmentId) }));
}

/** The tab a `?tab=` value opens, falling back like the URL tab hook does. */
export function environmentTab(environmentId: string, requested: string | null): EnvironmentResource | undefined {
	const visible = environmentTabItems(environmentId)
		.filter((item) => item.visible)
		.map((item) => item.value);
	const match = visible.find((tab) => tab === requested);
	if (match) return match;
	if (visible.includes(DEFAULT_ENVIRONMENT_TAB)) return DEFAULT_ENVIRONMENT_TAB;
	return visible[0];
}

/** First request of a tab's table, restored from its saved preferences. */
export function environmentTabRequest(tab: EnvironmentResource): SearchPaginationSortRequest {
	const key = ENVIRONMENT_TABLE_KEYS[tab];
	switch (tab) {
		case 'containers':
		case 'images':
			return resolveInitialListPageRequest(key, { column: 'created', direction: 'desc' });
		case 'projects':
			return withArchivedFilter(resolveInitialListPageRequest(key, { column: 'name', direction: 'asc' }), false);
		case 'volumes':
			return resolveInitialTableRequest(
				key,
				{ pagination: { page: 1, limit: 20 }, sort: { column: 'name', direction: 'asc' } },
				{ deferredSortColumns: ['size'] }
			);
		case 'networks':
			return resolveInitialListPageRequest(key, { column: 'name', direction: 'asc' });
	}
}

/** Warms the cache with a tab's first page; the tab renders a failure inline and retries it itself. */
export function preloadEnvironmentTab(
	queryClient: QueryClient,
	tab: EnvironmentResource,
	environment: EnvironmentRef
): Promise<unknown> {
	const environments = [environment];
	const request = environmentTabRequest(tab);
	const once = { revalidateIfStale: true, retry: false } as const;
	switch (tab) {
		case 'containers':
			return queryClient.ensureQueryData({
				...containerListQuery(environment.id, environments, { ...request, groupByProject: false }),
				...once
			});
		case 'projects':
			void queryClient.prefetchQuery({ ...projectStatusCountsQuery(environment.id, environments), retry: false });
			void queryClient.prefetchQuery({ ...projectTagsQuery(environment.id, environments), retry: false });
			return queryClient.ensureQueryData({ ...projectListQuery(environment.id, environments, request), ...once });
		case 'images':
			void queryClient.prefetchQuery({ ...imageUsageCountsQuery(environment.id, environments), retry: false });
			return Promise.all([
				queryClient.ensureQueryData({ ...imageListQuery(environment.id, environments, request), ...once }),
				featureStore.load([environment.id])
			]);
		case 'volumes':
			return queryClient.ensureQueryData({ ...volumeListQuery(environment.id, environments, request), ...once });
		case 'networks':
			return queryClient.ensureQueryData({ ...networkListQuery(environment.id, environments, request), ...once });
	}
}
