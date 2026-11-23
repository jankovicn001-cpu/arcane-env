<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createQuery } from '@tanstack/svelte-query';

	import type { ActionButtonMenuItem } from '#lib/components/action-button-group/types.js';
	import EnvironmentGroupSheet from '#lib/components/sheets/environment-group-sheet.svelte';
	import NewEnvironmentSheet from '#lib/components/sheets/new-environment-sheet.svelte';
	import { EmptyState } from '#lib/components/states/index.js';
	import { useEnvironmentGroupEditor } from '#lib/hooks/use-environment-group-editor.svelte.js';
	import { AddIcon, AgentIcon, EnvironmentsIcon, UsersIcon } from '#lib/icons/index.js';
	import { ResourcePageLayout, type ActionButton } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { dashboardContainerCountsQuery } from '#lib/query/dashboard.js';
	import { environmentGroupListQuery } from '#lib/query/environment-groups.js';
	import { environmentStore } from '#lib/stores/environment.store.svelte.js';
	import { GLOBAL_SCOPE } from '#lib/types/role.js';
	import { hasPermission, isGlobalAdmin } from '#lib/utils/auth.js';
	import { isEnvironmentOnline } from '#lib/utils/docker.js';
	import { browsableEnvironmentGroups, readableEnvironments, UNASSIGNED_GROUP_ID } from '#lib/utils/environments.js';

	import AgentCard from '../components/agent-card.svelte';
	import EnvironmentsBreadcrumb from '../components/environments-breadcrumb.svelte';
	import AddAgentsDialog from './components/add-agents-dialog.svelte';
	import EnvironmentAccessSheet from './components/environment-access-sheet.svelte';

	let { data } = $props();

	const editor = useEnvironmentGroupEditor({ onDeleted: () => goto(resolve('/(app)/environments')) });
	const managedGroupsQuery = createQuery(() => ({
		...environmentGroupListQuery(),
		enabled: hasPermission('environments:list', GLOBAL_SCOPE)
	}));
	// The live bucket, so agents joining, leaving or going offline, and renames, show without a reload.
	const group = $derived(
		browsableEnvironmentGroups(managedGroupsQuery.data).find((candidate) => candidate.id === data.group.id) ?? data.group
	);
	// Counts only come from agents whose containers the user may list.
	const countsQuery = createQuery(() =>
		dashboardContainerCountsQuery(readableEnvironments('containers:list', group.environments))
	);
	const countsById = $derived(
		new Map(countsQuery.data?.byEnvironment.map(({ environment, data: counts }) => [environment.id, counts]))
	);
	const onlineIds = $derived(new Set(environmentStore.available.filter(isEnvironmentOnline).map((env) => env.id)));

	let addAgentsOpen = $state(false);
	let addAgentsSession = $state(0);
	let newAgentOpen = $state(false);
	let newAgentSession = $state(0);
	let accessOpen = $state(false);
	let accessSession = $state(0);

	const actionButtons: ActionButton[] = $derived.by(() => {
		const buttons: ActionButton[] = [];
		if (group.id === UNASSIGNED_GROUP_ID) return buttons;
		const addMenu: ActionButtonMenuItem[] = [];
		if (editor.canUpdate) {
			addMenu.push({
				id: 'add-existing-agent',
				label: m.add_existing_agent(),
				icon: AgentIcon,
				onclick: () => {
					addAgentsSession += 1;
					addAgentsOpen = true;
				}
			});
		}
		if (editor.canCreate) {
			addMenu.push({
				id: 'add-new-agent',
				label: m.add_new_agent(),
				icon: AddIcon,
				onclick: () => {
					newAgentSession += 1;
					newAgentOpen = true;
				}
			});
		}
		// Putting agents in an environment extends its access grants, so only global admins may.
		if (addMenu.length > 0 && isGlobalAdmin()) {
			buttons.push({
				id: 'add-agent',
				action: 'create',
				placement: 'primary',
				label: m.common_add_button({ resource: m.resource_environment_cap() }),
				menuItems: addMenu
			});
		}
		// Role grants are reserved for global admins on the server.
		if (isGlobalAdmin()) {
			buttons.push({
				id: 'access',
				action: 'base',
				placement: 'secondary',
				icon: UsersIcon,
				label: m.access(),
				onclick: () => {
					accessSession += 1;
					accessOpen = true;
				}
			});
		}
		if (editor.canUpdate) {
			buttons.push({ id: 'edit', action: 'edit', label: m.common_edit(), onclick: () => editor.open(group) });
		}
		if (editor.canDelete) {
			buttons.push({
				id: 'delete',
				action: 'remove',
				group: 'danger',
				destructive: true,
				label: m.common_delete(),
				onclick: () => editor.confirmDelete(group),
				loading: editor.deletingId === group.id,
				disabled: editor.deletingId !== null
			});
		}
		return buttons;
	});
</script>

<div class="space-y-3">
	<EnvironmentsBreadcrumb current={group.name} />
	<ResourcePageLayout title={group.name} subtitle={group.description || undefined} {actionButtons}>
		{#snippet mainContent()}
			{#if group.environments.length === 0}
				<EmptyState
					icon={EnvironmentsIcon}
					title={m.environment_has_no_agents()}
					description={m.environment_has_no_agents_description()}
				/>
			{:else}
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
					{#each group.environments as environment (environment.id)}
						<AgentCard
							{environment}
							groupId={group.id}
							online={onlineIds.has(environment.id)}
							counts={countsById.get(environment.id)}
						/>
					{/each}
				</div>
			{/if}
		{/snippet}
	</ResourcePageLayout>
</div>

{#if editor.sheetSession > 0}
	{#key editor.sheetSession}
		<EnvironmentGroupSheet {editor} />
	{/key}
{/if}

{#if addAgentsSession > 0}
	{#key addAgentsSession}
		<AddAgentsDialog bind:open={addAgentsOpen} {group} />
	{/key}
{/if}

{#if accessSession > 0}
	{#key accessSession}
		<EnvironmentAccessSheet bind:open={accessOpen} {group} />
	{/key}
{/if}

{#if newAgentSession > 0}
	{#key newAgentSession}
		<NewEnvironmentSheet bind:open={newAgentOpen} groupId={group.id} />
	{/key}
{/if}
