<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { untrack } from 'svelte';

	import { EmptyState } from '#lib/components/states/index.js';
	import { ContainersIcon, ProjectsIcon, PlayIcon, StopIcon } from '#lib/icons/index.js';
	import { ResourcePageLayout, type ActionButton, type StatCardConfig } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { projectListQuery, projectStatusCountsQuery } from '#lib/query/projects.js';
	import { projectTagsQuery } from '#lib/query/projects.js';
	import { environmentStore } from '#lib/stores/environment.store.svelte.js';
	import type { ProjectStatusCounts } from '#lib/types/project.js';
	import { pinViewAction } from '#lib/utils/dashboard-layout.js';
	import { hasShownPermission, readableEnvironments, withFailedEnvironments } from '#lib/utils/environments.js';

	import ProjectsTable from './components/projects-table.svelte';
	import { withArchivedFilter } from './projects-table.helpers';

	let { data } = $props();

	let baseProjectRequestOptions = $state(untrack(() => withArchivedFilter(data.projectRequestOptions, data.showArchived)));
	let selectedIds = $state<string[]>([]);
	let isManualRefreshing = $state(false);
	const scope = $derived(environmentStore.shownKey);
	const showArchived = $derived(page.url.searchParams.get('archived') === 'true');
	const projectRequestOptions = $derived(withArchivedFilter(baseProjectRequestOptions, showArchived));
	const countsFallback: ProjectStatusCounts = {
		runningProjects: 0,
		stoppedProjects: 0,
		totalProjects: 0,
		archivedProjects: 0
	};

	// The load already cached the first page under its key; seeding every key with it would hide other pages and filters.
	const projectsQuery = createQuery(() => {
		const queryScope = scope;
		return {
			...projectListQuery(queryScope, readableEnvironments('projects:list'), projectRequestOptions),
			placeholderData: (previous, query) => {
				if (query?.queryKey[1] === queryScope) return previous;
				return undefined;
			},
			select: (value) => ({ envId: queryScope, value }),
			refetchOnMount: false
		};
	});
	let projects = $derived(projectsQuery.data?.envId === scope ? projectsQuery.data.value : null);

	const statusCountsQuery = createQuery(() => {
		const queryScope = scope;
		return {
			...projectStatusCountsQuery(queryScope, readableEnvironments('projects:list')),
			select: (value) => ({ envId: queryScope, value }),
			refetchOnMount: false
		};
	});
	const tagsQuery = createQuery(() => {
		const queryScope = scope;
		const environments = readableEnvironments('projects:list');
		return {
			...projectTagsQuery(queryScope, environments),
			select: (value) => ({ envId: queryScope, value }),
			refetchOnMount: false
		};
	});
	const projectTags = $derived(tagsQuery.data?.envId === scope ? tagsQuery.data.value.tags : []);
	const relatedFailures = $derived(
		[statusCountsQuery.data, tagsQuery.data].flatMap((result) => (result?.envId === scope ? result.value.failedEnvironments : []))
	);
	// Tags change rarely, so a refresh reloads them only after an environment failed to return them.
	const refetchFailedTags = () =>
		tagsQuery.isError || tagsQuery.data?.value.failedEnvironments.length ? tagsQuery.refetch() : undefined;

	const projectStatusCounts = $derived(statusCountsQuery.data?.envId === scope ? statusCountsQuery.data.value : countsFallback);
	const totalCompose = $derived(projectStatusCounts.totalProjects);
	const runningCompose = $derived(projectStatusCounts.runningProjects);
	const stoppedCompose = $derived(projectStatusCounts.stoppedProjects);
	const archivedCompose = $derived(projectStatusCounts.archivedProjects);
	const isRefreshBlocked = $derived(isManualRefreshing || projectsQuery.isFetching || statusCountsQuery.isFetching);

	async function refreshCompose() {
		if (isRefreshBlocked) return;
		const requestedScope = scope;
		isManualRefreshing = true;
		try {
			await Promise.all([projectsQuery.refetch(), statusCountsQuery.refetch(), refetchFailedTags()]);
		} finally {
			if (requestedScope === scope) {
				isManualRefreshing = false;
			}
		}
	}

	async function toggleArchived(next: boolean) {
		const url = new URL(page.url.href);
		if (next) {
			url.searchParams.set('archived', 'true');
		} else {
			url.searchParams.delete('archived');
		}
		await goto(`${resolve('/(app)/projects')}${url.searchParams.size > 0 ? `?${url.searchParams}` : ''}`, {
			reset: false,
			replace: true
		});
	}

	const canCreateProject = $derived(hasShownPermission('projects:create'));

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (canCreateProject) {
			buttons.push({
				id: 'create',
				action: 'create',
				placement: 'primary',
				label: m.compose_create_project(),
				onclick: () => goto(resolve('/(app)/projects/new'))
			});
		}
		buttons.push(pinViewAction('projects', projectRequestOptions, readableEnvironments('projects:list')));
		buttons.push({
			id: 'refresh',
			action: 'refresh',
			placement: 'secondary',
			iconOnly: true,
			label: m.common_refresh(),
			onclick: refreshCompose,
			loading: isManualRefreshing,
			disabled: isRefreshBlocked
		});
		return buttons;
	});

	const statCards: StatCardConfig[] = $derived([
		{
			title: m.common_total(),
			value: totalCompose,
			icon: ProjectsIcon,
			iconColor: 'text-warning'
		},
		{
			title: m.common_running(),
			value: runningCompose,
			icon: PlayIcon,
			iconColor: 'text-success'
		},
		{
			title: m.common_stopped(),
			value: stoppedCompose,
			icon: StopIcon,
			iconColor: 'text-destructive'
		},
		{
			title: m.projects_archived_badge(),
			value: archivedCompose,
			icon: ContainersIcon,
			iconColor: 'text-muted-foreground'
		}
	]);
</script>

{#snippet projectsEmptyState()}
	<EmptyState icon={ProjectsIcon} title={m.projects_empty_title()} description={m.projects_empty_description()} />
{/snippet}

<ResourcePageLayout title={m.projects_title()} subtitle={m.compose_subtitle()} {actionButtons} {statCards}>
	{#snippet mainContent()}
		{#if projects}
			<ProjectsTable
				projects={withFailedEnvironments(projects, relatedFailures)}
				emptyState={projectsEmptyState}
				bind:selectedIds
				bind:requestOptions={
					() => projectRequestOptions, (options) => (baseProjectRequestOptions = withArchivedFilter(options, showArchived))
				}
				{showArchived}
				availableTags={projectTags}
				onToggleArchived={toggleArchived}
				onRetry={() => Promise.all([projectsQuery.refetch(), statusCountsQuery.refetch(), refetchFailedTags()])}
				refreshError={projectsQuery.error}
			/>
		{/if}
	{/snippet}
</ResourcePageLayout>
