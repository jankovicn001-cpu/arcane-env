<script lang="ts">
	import { untrack } from 'svelte';
	import { z } from 'zod/v4';

	import FormInput from '#lib/components/form/form-input.svelte';
	import SheetFooterActions from '#lib/components/sheets/sheet-footer-actions.svelte';
	import * as ResponsiveDialog from '#lib/components/ui/responsive-dialog/index.js';
	import type { EnvironmentGroupEditor } from '#lib/hooks/use-environment-group-editor.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { applyApiFieldError } from '#lib/utils/api.js';
	import { createForm, preventDefault } from '#lib/utils/settings.svelte.js';

	let { editor }: { editor: EnvironmentGroupEditor } = $props();

	// Each sheet session is keyed, so the group it opened with stays fixed.
	const group = untrack(() => editor.editing);
	const isSubmitting = $derived(editor.isSaving);

	// Matches the backend's limits on environment groups.
	const groupForm = createForm(
		z.object({
			name: z
				.string()
				.min(1, m.common_name_required())
				.max(64, m.common_max_length({ field: m.common_name(), maxLength: 64 })),
			description: z.string().max(256, m.common_max_length({ field: m.common_description(), maxLength: 256 }))
		}),
		{ name: group?.name ?? '', description: group?.description ?? '' }
	);
	const groupInputs = $derived(groupForm.inputs);

	async function handleSubmit() {
		const data = groupForm.validate();
		if (!data) return;
		applyApiFieldError(groupInputs, await editor.submit(data));
	}
</script>

<ResponsiveDialog.Root
	bind:open={() => editor.sheetOpen, (open) => (editor.sheetOpen = open)}
	variant="sheet"
	title={group ? m.edit_resource({ resource: m.environment() }) : m.common_create_button({ resource: m.environment() })}
	description={m.group_agents_description()}
	contentClass="sm:max-w-125"
>
	<form id="environment-group-form" onsubmit={preventDefault(handleSubmit)} class="grid gap-4 py-6">
		<FormInput label={m.common_name()} placeholder={m.environment_name_placeholder()} bind:input={groupInputs.name} />
		<FormInput label={m.common_description()} type="textarea" bind:input={groupInputs.description} />
	</form>

	{#snippet footer()}
		<SheetFooterActions
			bind:open={() => editor.sheetOpen, (open) => (editor.sheetOpen = open)}
			cancelDisabled={isSubmitting}
			submitAction={group ? 'save' : 'create'}
			submitLabel={group ? m.common_save() : m.common_create()}
			submitDisabled={isSubmitting}
			submitLoading={isSubmitting}
			submitForm="environment-group-form"
		/>
	{/snippet}
</ResponsiveDialog.Root>
