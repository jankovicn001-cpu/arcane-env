<script lang="ts">
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { SvelteSet } from 'svelte/reactivity';

	import NewEnvironmentSheet from '#lib/components/sheets/new-environment-sheet.svelte';
	import { TabBar } from '#lib/components/tab-bar/index.js';
	import * as Tabs from '#lib/components/ui/tabs/index.js';
	import { useUrlTab } from '#lib/hooks/use-url-tab.svelte.js';
	import { AgentIcon, ApiKeyIcon, DownloadIcon, EnvironmentsIcon } from '#lib/icons/index.js';
	import { ResourcePageLayout, type ActionButton } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { environmentListQuery } from '#lib/query/environments.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { environmentManagementService } from '#lib/services/env-mgmt-service.js';
	import settingsStore from '#lib/stores/config-store.svelte.js';
	import { environmentStore } from '#lib/stores/environment.store.svelte.js';
	import { GLOBAL_SCOPE } from '#lib/types/role.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { hasPermission } from '#lib/utils/auth.js';
	import { downloadFromUrl } from '#lib/utils/browser-download.js';
	import { bulkConfirmAndRun } from '#lib/utils/bulk-actions.js';
	import { ResourceListPageState } from '#lib/utils/resource-list-page.svelte.js';

	import EnrollmentTokensTab from './components/enrollment-tokens-tab.svelte';
	import EnvironmentGroupsTab from './components/environment-groups-tab.svelte';
	import EnvironmentTable from './components/environment-table.svelte';

	let { data } = $props();
	const queryClient = useQueryClient();

	const pageState = new ResourceListPageState(untrack(() => data.environmentRequestOptions));
	const environmentsQuery = createQuery(() => ({
		...environmentListQuery(pageState.requestOptions),
		placeholderData: (previous) => previous
	}));
	const loadedEnvironments = $derived(environmentsQuery.data ?? data.environments);
	const liveEnvironments = $derived(new Map(environmentStore.available.map((environment) => [environment.id, environment])));
	const environments = $derived({
		...loadedEnvironments,
		data: loadedEnvironments.data.map((environment) => liveEnvironments.get(environment.id) ?? environment)
	});
	let environmentSession = $state(0);
	let isDeleting = $state(false);
	const isRefreshing = $derived(environmentsQuery.isFetching && !environmentsQuery.isPending);

	async function refresh() {
		const result = await environmentsQuery.refetch();
		if (result.isError) {
			toast.error(m.common_refresh_failed({ resource: m.environments_title() }), {
				description: extractApiErrorMessage(result.error)
			});
		}
	}

	const canCreateEnvironment = $derived(hasPermission('environments:create', GLOBAL_SCOPE));
	const canDeleteEnvironment = $derived(hasPermission('environments:delete', GLOBAL_SCOPE));
	const canListEnvironmentGroups = $derived(hasPermission('environments:list', GLOBAL_SCOPE));

	const urlTab = useUrlTab({
		tabs: () => [
			{
				value: 'environments',
				label: m.environments_title(),
				icon: AgentIcon
			},
			{
				value: 'environment-groups',
				label: m.environments(),
				icon: EnvironmentsIcon,
				visible: canListEnvironmentGroups
			},
			{
				value: 'enrollment-tokens',
				label: m.enrollment_tokens(),
				icon: ApiKeyIcon,
				visible: canCreateEnvironment
			}
		],
		defaultTab: () => 'environments'
	});

	function handleBulkDelete(ids: string[] = pageState.selectedIds) {
		if (ids.length === 0) return;
		const environmentIds = [...ids];
		const removedIds = new SvelteSet<string>();

		bulkConfirmAndRun({
			ids: environmentIds,
			title: m.environments_remove_selected_title({ count: environmentIds.length }),
			message: m.environments_remove_selected_message(),
			confirmLabel: m.common_remove(),
			destructive: true,
			run: async (id) => {
				await environmentManagementService.delete(id);
				removedIds.add(id);
			},
			messages: {
				success: (count) => m.common_bulk_remove_success({ count, resource: m.environments_title() }),
				partial: (success, total, failed) =>
					m.common_bulk_remove_partial({ success, total, failed, resource: m.environments_title() }),
				failure: () => m.common_bulk_remove_failed({ count: environmentIds.length, resource: m.environments_title() })
			},
			setLoading: (loading) => (isDeleting = loading),
			onComplete: async ({ success }) => {
				if (success === 0) return;
				await environmentStore.initialize(environmentStore.available.filter((environment) => !removedIds.has(environment.id)));
				await queryClient.invalidateQueries({ queryKey: queryKeys.environments.all });
			},
			clearSelection: () => (pageState.selectedIds = []),
			sequential: true
		});
	}

	function onEnvironmentCreated() {
		pageState.isCreateDialogOpen = false;
		toast.success(m.common_create_success({ resource: m.resource_environment() }));
	}

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (pageState.selectedIds.length > 0 && canDeleteEnvironment) {
			buttons.push({
				id: 'remove-selected',
				action: 'remove',
				group: 'danger',
				destructive: true,
				label: m.common_remove_selected(),
				onclick: handleBulkDelete,
				loading: isDeleting,
				disabled: isDeleting
			});
		}
		if (canCreateEnvironment) {
			buttons.push({
				id: 'create',
				action: 'create',
				placement: 'primary',
				label: m.common_add_button({ resource: m.resource_environment_cap() }),
				onclick: () => {
					environmentSession += 1;
					pageState.isCreateDialogOpen = true;
				}
			});
		}
		if (canCreateEnvironment && settingsStore.runtime.agentMTLSManagerCAAvailable) {
			buttons.push({
				id: 'download-agent-ca',
				action: 'save',
				label: m.environments_download_agent_ca(),
				onclick: () => downloadFromUrl('/api/agent/mtls/ca'),
				icon: DownloadIcon
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
			disabled: environmentsQuery.isFetching
		});
		return buttons;
	});
</script>

<ResourcePageLayout title={m.environments_title()} subtitle={m.environments_subtitle()} {actionButtons}>
	{#snippet mainContent()}
		<Tabs.Root value={urlTab.value}>
			<div class="pb-6">
				<TabBar items={urlTab.items} onValueChange={urlTab.select} />
			</div>

			<Tabs.Content value="environments">
				<EnvironmentTable
					{environments}
					bind:selectedIds={pageState.selectedIds}
					bind:requestOptions={pageState.requestOptions}
					onRetry={() => environmentsQuery.refetch()}
					onDeleteSelected={handleBulkDelete}
					bulkDeleteLoading={isDeleting}
				/>
			</Tabs.Content>

			{#if canListEnvironmentGroups}
				<Tabs.Content value="environment-groups">
					<EnvironmentGroupsTab />
				</Tabs.Content>
			{/if}

			{#if canCreateEnvironment}
				<Tabs.Content value="enrollment-tokens">
					<EnrollmentTokensTab />
				</Tabs.Content>
			{/if}
		</Tabs.Root>
	{/snippet}

	{#snippet additionalContent()}
		{#if environmentSession > 0}
			{#key environmentSession}
				<NewEnvironmentSheet bind:open={pageState.isCreateDialogOpen} {onEnvironmentCreated} />
			{/key}
		{/if}
	{/snippet}
</ResourcePageLayout>
