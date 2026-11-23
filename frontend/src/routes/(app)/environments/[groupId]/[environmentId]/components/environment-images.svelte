<script lang="ts">
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import type { Snippet } from 'svelte';

	import ImagePullSheet from '#lib/components/sheets/image-pull-sheet.svelte';
	import { LocalFolderComputerIcon, VolumesIcon } from '#lib/icons/index.js';
	import type { ActionButton, StatCardConfig } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { EMPTY_IMAGE_COUNTS } from '#lib/query/dashboard.js';
	import { imageListQuery, imageUsageCountsQuery } from '#lib/query/images.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
	import { hasPermission } from '#lib/utils/auth.js';
	import { withFailedEnvironments } from '#lib/utils/environments.js';
	import { bytes } from '#lib/utils/formatting.js';

	import ImageTable from '../../../../images/components/image-table.svelte';
	import { ENVIRONMENT_TABLE_KEYS, environmentTabRequest, type EnvironmentReachability } from '../environment-tabs';
	import EnvironmentTabLayout from './environment-tab-layout.svelte';

	let { environment, status, tabs }: { environment: EnvironmentRef; status: EnvironmentReachability; tabs: Snippet } = $props();

	const queryClient = useQueryClient();
	const environments = $derived([environment]);

	let requestOptions = $state(environmentTabRequest('images'));
	let selectedIds = $state<string[]>([]);
	let isPullSheetOpen = $state(false);

	const imagesQuery = createQuery(() => ({
		...imageListQuery(environment.id, environments, requestOptions),
		enabled: status.online,
		placeholderData: (previous) => previous
	}));
	const usageCountsQuery = createQuery(() => ({
		...imageUsageCountsQuery(environment.id, environments),
		enabled: status.online
	}));

	const images = $derived(imagesQuery.data);
	const usage = $derived(usageCountsQuery.data ?? EMPTY_IMAGE_COUNTS);
	const isRefreshing = $derived(imagesQuery.isFetching || usageCountsQuery.isFetching);

	function invalidateImages() {
		return queryClient.invalidateQueries({ queryKey: queryKeys.images.listPrefix(environment.id) });
	}

	function refresh() {
		return Promise.all([imagesQuery.refetch(), usageCountsQuery.refetch()]);
	}

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (hasPermission('images:pull', environment.id)) {
			buttons.push({
				id: 'pull',
				action: 'pull',
				placement: 'primary',
				label: m.images_pull_image(),
				onclick: () => (isPullSheetOpen = true)
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
		{
			title: usage.totalImages === 1 ? m.images_total_one() : m.images_total(),
			value: usage.totalImages,
			icon: VolumesIcon,
			iconColor: 'text-info'
		},
		{
			title: m.images_total_size(),
			value: String(bytes.format(usage.totalImageSize, { unitSeparator: ' ' })),
			icon: LocalFolderComputerIcon,
			iconColor: 'text-warning'
		}
	]);
</script>

<EnvironmentTabLayout
	{environment}
	{status}
	subtitle={m.environment_images_subtitle()}
	{actionButtons}
	{statCards}
	query={imagesQuery}
	{tabs}
>
	{#if images}
		<ImageTable
			images={withFailedEnvironments(images, usageCountsQuery.data?.failedEnvironments)}
			{environments}
			persistKey={ENVIRONMENT_TABLE_KEYS.images}
			bind:selectedIds
			bind:requestOptions
			onRetry={refresh}
			refreshError={imagesQuery.error}
		/>
	{/if}

	{#snippet additionalContent()}
		<ImagePullSheet bind:open={isPullSheetOpen} {environments} onPullFinished={invalidateImages} />
	{/snippet}
</EnvironmentTabLayout>
