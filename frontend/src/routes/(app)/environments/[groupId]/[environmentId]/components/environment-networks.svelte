<script lang="ts">
	import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';

	import CreateNetworkSheet from '#lib/components/sheets/create-network-sheet.svelte';
	import { ConnectionIcon, NetworksIcon } from '#lib/icons/index.js';
	import type { ActionButton, StatCardConfig } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { networkListQuery } from '#lib/query/networks.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { networkService } from '#lib/services/network-service.js';
	import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
	import type { NetworkCreateOptions } from '#lib/types/network.js';
	import { activityToastOptions, extractActivityId } from '#lib/utils/activity-toast.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { hasPermission } from '#lib/utils/auth.js';

	import NetworkTable from '../../../../networks/components/network-table.svelte';
	import { ENVIRONMENT_TABLE_KEYS, environmentTabRequest, type EnvironmentReachability } from '../environment-tabs';
	import EnvironmentTabLayout from './environment-tab-layout.svelte';

	let { environment, status, tabs }: { environment: EnvironmentRef; status: EnvironmentReachability; tabs: Snippet } = $props();

	const queryClient = useQueryClient();
	const environments = $derived([environment]);

	let requestOptions = $state(environmentTabRequest('networks'));
	let selectedIds = $state<string[]>([]);
	let isCreateSheetOpen = $state(false);

	const networksQuery = createQuery(() => ({
		...networkListQuery(environment.id, environments, requestOptions),
		enabled: status.online,
		placeholderData: (previous) => previous
	}));

	const networks = $derived(networksQuery.data);
	const counts = $derived(networks?.counts ?? { inuse: 0, unused: 0, total: 0 });
	const isRefreshing = $derived(networksQuery.isFetching);

	const createNetworkMutation = createMutation(() => ({
		mutationKey: ['networks', 'create', environment.id],
		mutationFn: ({ name, options, environmentId }: { name: string; options: NetworkCreateOptions; environmentId: string }) =>
			networkService.createNetwork(name, options, environmentId),
		onSuccess: async (data, { name }) => {
			toast.success(
				m.common_create_success({ resource: `${m.resource_network()} "${name}"` }),
				activityToastOptions(extractActivityId(data))
			);
			await queryClient.invalidateQueries({ queryKey: queryKeys.networks.listPrefix(environment.id) });
			isCreateSheetOpen = false;
		},
		onError: (error, { name }) => {
			toast.error(m.common_create_failed({ resource: `${m.resource_network()} "${name}"` }), {
				description: extractApiErrorMessage(error)
			});
		}
	}));

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (hasPermission('networks:create', environment.id)) {
			buttons.push({
				id: 'create',
				action: 'create',
				placement: 'primary',
				label: m.common_create_button({ resource: m.resource_network_cap() }),
				onclick: () => (isCreateSheetOpen = true),
				loading: createNetworkMutation.isPending,
				disabled: createNetworkMutation.isPending
			});
		}
		buttons.push({
			id: 'refresh',
			action: 'refresh',
			placement: 'secondary',
			iconOnly: true,
			label: m.common_refresh(),
			onclick: () => networksQuery.refetch(),
			loading: isRefreshing,
			disabled: isRefreshing
		});
		return buttons;
	});

	const statCards: StatCardConfig[] = $derived([
		{
			title: counts.total === 1 ? m.networks_total_one() : m.networks_total(),
			value: counts.total,
			icon: NetworksIcon,
			iconColor: 'text-info'
		},
		{
			title: counts.unused === 1 ? m.unused_networks_one() : m.unused_networks(),
			value: counts.unused,
			icon: ConnectionIcon,
			iconColor: 'text-warning'
		}
	]);
</script>

<EnvironmentTabLayout
	{environment}
	{status}
	subtitle={m.environment_networks_subtitle()}
	{actionButtons}
	{statCards}
	query={networksQuery}
	{tabs}
>
	{#if networks}
		<NetworkTable
			{networks}
			{environments}
			persistKey={ENVIRONMENT_TABLE_KEYS.networks}
			bind:selectedIds
			bind:requestOptions
			onRetry={() => networksQuery.refetch()}
			refreshError={networksQuery.error}
		/>
	{/if}

	{#snippet additionalContent()}
		<CreateNetworkSheet
			bind:open={isCreateSheetOpen}
			{environments}
			isLoading={createNetworkMutation.isPending}
			onSubmit={(name, options, environmentId) => createNetworkMutation.mutate({ name, options, environmentId })}
		/>
	{/snippet}
</EnvironmentTabLayout>
