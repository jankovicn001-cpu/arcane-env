<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';

	import SearchInput from '#lib/components/form/search-input.svelte';
	import EnvironmentGroupSheet from '#lib/components/sheets/environment-group-sheet.svelte';
	import { EmptyState } from '#lib/components/states/index.js';
	import { useEnvironmentGroupEditor } from '#lib/hooks/use-environment-group-editor.svelte.js';
	import { EnvironmentsIcon } from '#lib/icons/index.js';
	import { ResourcePageLayout, type ActionButton } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { environmentGroupListQuery } from '#lib/query/environment-groups.js';
	import { environmentStore } from '#lib/stores/environment.store.svelte.js';
	import { GLOBAL_SCOPE } from '#lib/types/role.js';
	import { hasPermission } from '#lib/utils/auth.js';
	import { isEnvironmentOnline } from '#lib/utils/docker.js';
	import { browsableEnvironmentGroups } from '#lib/utils/environments.js';

	import EnvironmentGroupCard from './components/environment-group-card.svelte';

	const editor = useEnvironmentGroupEditor();
	// Admins also see groups without agents, so a group they just created shows up.
	const managedGroupsQuery = createQuery(() => ({
		...environmentGroupListQuery(),
		enabled: hasPermission('environments:list', GLOBAL_SCOPE)
	}));
	const groups = $derived(browsableEnvironmentGroups(managedGroupsQuery.data));

	const actionButtons: ActionButton[] = $derived(
		editor.canCreate
			? [
					{
						id: 'create',
						action: 'create',
						placement: 'primary',
						label: m.common_create_button({ resource: m.environment() }),
						onclick: () => editor.open(null)
					}
				]
			: []
	);
	const onlineIds = $derived(new Set(environmentStore.available.filter(isEnvironmentOnline).map((env) => env.id)));

	let search = $state('');
	const matching = $derived.by(() => {
		const term = search.trim().toLowerCase();
		if (!term) return groups;
		return groups.filter(
			(group) =>
				group.name.toLowerCase().includes(term) || group.environments.some((env) => env.name.toLowerCase().includes(term))
		);
	});
</script>

<ResourcePageLayout title={m.environments()} subtitle={m.environment_resources_description()} {actionButtons}>
	{#snippet mainContent()}
		{#if groups.length === 0}
			<EmptyState
				icon={EnvironmentsIcon}
				title={m.no_environments_to_browse()}
				description={m.no_environments_to_browse_description()}
			/>
		{:else}
			<div class="flex flex-col gap-6">
				<div class="max-w-sm">
					<SearchInput bind:value={search} onClear={() => (search = '')} aria-label={m.search()} />
				</div>
				{#if matching.length === 0}
					<EmptyState icon={EnvironmentsIcon} title={m.common_no_results_found()} description={m.common_no_results_hint()} />
				{:else}
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
						{#each matching as group (group.id)}
							<EnvironmentGroupCard {group} online={group.environments.filter((env) => onlineIds.has(env.id)).length} />
						{/each}
					</div>
				{/if}
			</div>
		{/if}
	{/snippet}
</ResourcePageLayout>

{#if editor.sheetSession > 0}
	{#key editor.sheetSession}
		<EnvironmentGroupSheet {editor} />
	{/key}
{/if}
