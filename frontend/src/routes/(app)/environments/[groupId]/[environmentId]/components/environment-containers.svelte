<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createQuery } from '@tanstack/svelte-query';
	import type { Snippet } from 'svelte';

	import { ContainersIcon } from '#lib/icons/index.js';
	import type { ActionButton, StatCardConfig } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { containerListQuery } from '#lib/query/containers.js';
	import { EMPTY_CONTAINER_COUNTS } from '#lib/query/dashboard.js';
	import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
	import { hasPermission } from '#lib/utils/auth.js';

	import ContainerTable from '../../../../containers/components/container-table.svelte';
	import { ENVIRONMENT_TABLE_KEYS, environmentTabRequest, type EnvironmentReachability } from '../environment-tabs';
	import EnvironmentTabLayout from './environment-tab-layout.svelte';

	let { environment, status, tabs }: { environment: EnvironmentRef; status: EnvironmentReachability; tabs: Snippet } = $props();

	const environments = $derived([environment]);

	let requestOptions = $state(environmentTabRequest('containers'));
	let selectedIds = $state<string[]>([]);
	let groupByProject = $state(false);

	const containersQuery = createQuery(() => ({
		...containerListQuery(environment.id, environments, { ...requestOptions, groupByProject }),
		enabled: status.online,
		placeholderData: (previous) => previous
	}));

	const containers = $derived(containersQuery.data);
	const counts = $derived(containers?.counts ?? EMPTY_CONTAINER_COUNTS);
	const isRefreshing = $derived(containersQuery.isFetching && !containersQuery.isPending);

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (hasPermission('containers:create', environment.id)) {
			buttons.push({
				id: 'create',
				action: 'create',
				placement: 'primary',
				label: m.common_create_button({ resource: m.container() }),
				onclick: () => goto(resolve(`/(app)/containers/new?env=${encodeURIComponent(environment.id)}`))
			});
		}
		buttons.push({
			id: 'refresh',
			action: 'refresh',
			placement: 'secondary',
			iconOnly: true,
			label: m.common_refresh(),
			onclick: () => containersQuery.refetch(),
			loading: isRefreshing,
			disabled: isRefreshing
		});
		return buttons;
	});

	const statCards: StatCardConfig[] = $derived([
		{ title: m.common_total(), value: counts.totalContainers, icon: ContainersIcon, iconColor: 'text-info' },
		{ title: m.common_running(), value: counts.runningContainers, icon: ContainersIcon, iconColor: 'text-success' },
		{ title: m.common_stopped(), value: counts.stoppedContainers, icon: ContainersIcon, iconColor: 'text-destructive' }
	]);
</script>

<EnvironmentTabLayout
	{environment}
	{status}
	subtitle={m.environment_containers_subtitle()}
	{actionButtons}
	{statCards}
	query={containersQuery}
	{tabs}
>
	{#if containers}
		<ContainerTable
			scope={environment.id}
			{environments}
			persistKey={ENVIRONMENT_TABLE_KEYS.containers}
			{containers}
			bind:selectedIds
			bind:requestOptions
			bind:groupByProject
			loading={containersQuery.isPlaceholderData}
			onRetry={() => containersQuery.refetch()}
			refreshError={containersQuery.error}
		/>
	{/if}
</EnvironmentTabLayout>
