<script lang="ts">
	import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';

	import CreateVolumeSheet from '#lib/components/sheets/create-volume-sheet.svelte';
	import { VolumesIcon, VolumeUnusedIcon } from '#lib/icons/index.js';
	import type { ActionButton, StatCardConfig } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { volumeListQuery } from '#lib/query/volumes.js';
	import { volumeService } from '#lib/services/volume-service.js';
	import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
	import type { VolumeCreate } from '#lib/types/ui/volume.js';
	import { activityToastOptions, extractActivityId } from '#lib/utils/activity-toast.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { hasPermission } from '#lib/utils/auth.js';

	import VolumeTable from '../../../../volumes/components/volume-table.svelte';
	import { ENVIRONMENT_TABLE_KEYS, environmentTabRequest, type EnvironmentReachability } from '../environment-tabs';
	import EnvironmentTabLayout from './environment-tab-layout.svelte';

	let { environment, status, tabs }: { environment: EnvironmentRef; status: EnvironmentReachability; tabs: Snippet } = $props();

	const queryClient = useQueryClient();
	const environments = $derived([environment]);

	let requestOptions = $state(environmentTabRequest('volumes'));
	let selectedIds = $state<string[]>([]);
	let isCreateSheetOpen = $state(false);

	const volumesQuery = createQuery(() => ({
		...volumeListQuery(environment.id, environments, requestOptions),
		enabled: status.online,
		placeholderData: (previous) => previous
	}));

	const volumes = $derived(volumesQuery.data);
	const counts = $derived(volumes?.counts ?? { inuse: 0, unused: 0, total: 0 });
	const isRefreshing = $derived(volumesQuery.isFetching);

	const createVolumeMutation = createMutation(() => ({
		mutationKey: ['volumes', 'create', environment.id],
		mutationFn: ({ options, environmentId }: { options: VolumeCreate; environmentId: string }) =>
			volumeService.createVolume(options, environmentId),
		onSuccess: async (data, { options }) => {
			const resource = `${m.resource_volume()} "${options.name?.trim() || m.common_unknown()}"`;
			toast.success(m.common_create_success({ resource }), activityToastOptions(extractActivityId(data)));
			await queryClient.invalidateQueries({ queryKey: queryKeys.volumes.listPrefix(environment.id) });
			isCreateSheetOpen = false;
		},
		onError: (error, { options }) => {
			const resource = `${m.resource_volume()} "${options.name?.trim() || m.common_unknown()}"`;
			toast.error(m.common_create_failed({ resource }), { description: extractApiErrorMessage(error) });
		}
	}));

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (hasPermission('volumes:create', environment.id)) {
			buttons.push({
				id: 'create',
				action: 'create',
				placement: 'primary',
				label: m.common_create_button({ resource: m.resource_volume_cap() }),
				onclick: () => (isCreateSheetOpen = true),
				loading: createVolumeMutation.isPending,
				disabled: createVolumeMutation.isPending
			});
		}
		buttons.push({
			id: 'refresh',
			action: 'refresh',
			placement: 'secondary',
			iconOnly: true,
			label: m.common_refresh(),
			onclick: () => volumesQuery.refetch(),
			loading: isRefreshing,
			disabled: isRefreshing
		});
		return buttons;
	});

	const statCards: StatCardConfig[] = $derived([
		{
			title: counts.total === 1 ? m.volumes_stat_total_one() : m.volumes_stat_total(),
			value: counts.total,
			icon: VolumesIcon,
			iconColor: 'text-info'
		},
		{
			title: counts.unused === 1 ? m.unused_volumes_one() : m.unused_volumes(),
			value: counts.unused,
			icon: VolumeUnusedIcon,
			iconColor: 'text-warning'
		}
	]);
</script>

<EnvironmentTabLayout
	{environment}
	{status}
	subtitle={m.environment_volumes_subtitle()}
	{actionButtons}
	{statCards}
	query={volumesQuery}
	{tabs}
>
	{#if volumes}
		<VolumeTable
			{volumes}
			{environments}
			persistKey={ENVIRONMENT_TABLE_KEYS.volumes}
			bind:selectedIds
			bind:requestOptions
			onRetry={() => volumesQuery.refetch()}
			refreshError={volumesQuery.error}
		/>
	{/if}

	{#snippet additionalContent()}
		<CreateVolumeSheet
			bind:open={isCreateSheetOpen}
			{environments}
			isLoading={createVolumeMutation.isPending}
			onSubmit={(options, environmentId) => createVolumeMutation.mutate({ options, environmentId })}
		/>
	{/snippet}
</EnvironmentTabLayout>
