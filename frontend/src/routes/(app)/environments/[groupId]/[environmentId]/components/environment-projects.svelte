<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createQuery } from '@tanstack/svelte-query';
	import type { Snippet } from 'svelte';

	import { EmptyState } from '#lib/components/states/index.js';
	import { ContainersIcon, PlayIcon, ProjectsIcon, StopIcon } from '#lib/icons/index.js';
	import type { ActionButton, StatCardConfig } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { emptyStatusCounts, projectListQuery, projectStatusCountsQuery, projectTagsQuery } from '#lib/query/projects.js';
	import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
	import { hasPermission } from '#lib/utils/auth.js';
	import { withFailedEnvironments } from '#lib/utils/environments.js';

	import ProjectsTable from '../../../../projects/components/projects-table.svelte';
	import { withArchivedFilter } from '../../../../projects/projects-table.helpers';
	import { ENVIRONMENT_TABLE_KEYS, environmentTabRequest, type EnvironmentReachability } from '../environment-tabs';
	import EnvironmentTabLayout from './environment-tab-layout.svelte';

	let { environment, status, tabs }: { environment: EnvironmentRef; status: EnvironmentReachability; tabs: Snippet } = $props();

	const environments = $derived([environment]);

	let baseRequestOptions = $state(environmentTabRequest('projects'));
	let showArchived = $state(false);
	let selectedIds = $state<string[]>([]);
	const requestOptions = $derived(withArchivedFilter(baseRequestOptions, showArchived));

	const projectsQuery = createQuery(() => ({
		...projectListQuery(environment.id, environments, requestOptions),
		enabled: status.online,
		placeholderData: (previous) => previous
	}));
	const statusCountsQuery = createQuery(() => ({
		...projectStatusCountsQuery(environment.id, environments),
		enabled: status.online
	}));
	const tagsQuery = createQuery(() => ({ ...projectTagsQuery(environment.id, environments), enabled: status.online }));

	const projects = $derived(projectsQuery.data);
	const counts = $derived(statusCountsQuery.data ?? emptyStatusCounts);
	const relatedFailures = $derived(
		[statusCountsQuery.data, tagsQuery.data].flatMap((result) => result?.failedEnvironments ?? [])
	);
	const isRefreshing = $derived(projectsQuery.isFetching || statusCountsQuery.isFetching);

	function refresh() {
		return Promise.all([projectsQuery.refetch(), statusCountsQuery.refetch(), tagsQuery.refetch()]);
	}

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (hasPermission('projects:create', environment.id)) {
			buttons.push({
				id: 'create',
				action: 'create',
				placement: 'primary',
				label: m.compose_create_project(),
				onclick: () => goto(resolve(`/(app)/projects/new?env=${encodeURIComponent(environment.id)}`))
			});
		}
		buttons.push({
			id: 'refresh',
			action: 'refresh',
			placement: 'secondary',
			iconOnly: true,
			label: m.common_refresh(),
			onclick: refresh,
			loading: isRefreshing,
			disabled: isRefreshing
		});
		return buttons;
	});

	const statCards: StatCardConfig[] = $derived([
		{ title: m.common_total(), value: counts.totalProjects, icon: ProjectsIcon, iconColor: 'text-warning' },
		{ title: m.common_running(), value: counts.runningProjects, icon: PlayIcon, iconColor: 'text-success' },
		{ title: m.common_stopped(), value: counts.stoppedProjects, icon: StopIcon, iconColor: 'text-destructive' },
		{
			title: m.projects_archived_badge(),
			value: counts.archivedProjects,
			icon: ContainersIcon,
			iconColor: 'text-muted-foreground'
		}
	]);
</script>

{#snippet projectsEmptyState()}
	<EmptyState icon={ProjectsIcon} title={m.projects_empty_title()} description={m.projects_empty_description()} />
{/snippet}

<EnvironmentTabLayout
	{environment}
	{status}
	subtitle={m.environment_projects_subtitle()}
	{actionButtons}
	{statCards}
	query={projectsQuery}
	{tabs}
>
	{#if projects}
		<ProjectsTable
			projects={withFailedEnvironments(projects, relatedFailures)}
			{environments}
			persistKey={ENVIRONMENT_TABLE_KEYS.projects}
			emptyState={projectsEmptyState}
			bind:selectedIds
			bind:requestOptions={() => requestOptions, (options) => (baseRequestOptions = withArchivedFilter(options, showArchived))}
			{showArchived}
			availableTags={tagsQuery.data?.tags ?? []}
			onToggleArchived={(next) => {
				showArchived = next;
			}}
			onRetry={refresh}
			refreshError={projectsQuery.error}
		/>
	{/if}
</EnvironmentTabLayout>
