<script lang="ts">
	import { useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';

	import EnvironmentMultiSelect from '#lib/components/sheets/environment-multi-select.svelte';
	import SheetFooterActions from '#lib/components/sheets/sheet-footer-actions.svelte';
	import * as ResponsiveDialog from '#lib/components/ui/responsive-dialog/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { environmentManagementService } from '#lib/services/env-mgmt-service.js';
	import { environmentStore } from '#lib/stores/environment.store.svelte.js';
	import type { EnvironmentGroupRef } from '#lib/types/environment.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { preventDefault } from '#lib/utils/settings.svelte.js';
	import { tryCatch } from '#lib/utils/try-catch.js';

	let { open = $bindable(false), group }: { open: boolean; group: EnvironmentGroupRef } = $props();

	const queryClient = useQueryClient();
	const candidates = $derived(environmentStore.available.filter((env) => env.group?.id !== group.id));
	let selected = $state<string[]>([]);
	let isSubmitting = $state(false);

	async function handleSubmit() {
		if (selected.length === 0) return;
		isSubmitting = true;
		const outcomes = await Promise.all(
			selected.map(async (id) => ({ id, result: await tryCatch(environmentManagementService.update(id, { groupId: group.id })) }))
		);
		const moved = new Set(outcomes.filter(({ result }) => result.error === null).map(({ id }) => id));
		for (const { id, result } of outcomes) {
			if (result.error === null) continue;
			const name = environmentStore.available.find((env) => env.id === id)?.name ?? id;
			toast.error(m.agent_add_to_environment_failed({ name }), { description: extractApiErrorMessage(result.error) });
		}
		if (moved.size > 0) {
			// Patched until the live stream catches up, so the agents show here at once.
			await environmentStore.initialize(
				environmentStore.available.map((env) => {
					if (!moved.has(env.id)) return env;
					return { ...env, groupId: group.id, group: { id: group.id, name: group.name, description: group.description } };
				})
			);
			await Promise.all([
				queryClient.invalidateQueries({ queryKey: queryKeys.environments.all }),
				queryClient.invalidateQueries({ queryKey: queryKeys.environmentGroups.all })
			]);
			toast.success(m.agents_added_to_environment({ count: moved.size, name: group.name }));
		}
		isSubmitting = false;
		if (moved.size === selected.length) open = false;
		selected = selected.filter((id) => !moved.has(id));
	}
</script>

<ResponsiveDialog.Root
	bind:open
	variant="sheet"
	title={m.add_existing_agents()}
	description={m.add_existing_agents_description()}
	contentClass="sm:max-w-125"
>
	<form id="add-agents-form" onsubmit={preventDefault(handleSubmit)} class="py-6">
		{#if candidates.length === 0}
			<p class="text-sm text-muted-foreground">{m.no_agents_to_add()}</p>
		{:else}
			<EnvironmentMultiSelect environments={candidates} bind:selected disabled={isSubmitting} />
		{/if}
	</form>

	{#snippet footer()}
		<SheetFooterActions
			bind:open
			cancelDisabled={isSubmitting}
			submitAction="save"
			submitLabel={m.common_add()}
			submitDisabled={isSubmitting || selected.length === 0}
			submitLoading={isSubmitting}
			submitForm="add-agents-form"
		/>
	{/snippet}
</ResponsiveDialog.Root>
