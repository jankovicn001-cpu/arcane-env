<script lang="ts">
	import { untrack } from 'svelte';
	import { z } from 'zod/v4';

	import FormInput from '#lib/components/form/form-input.svelte';
	import RowListEditor from '#lib/components/form/row-list-editor.svelte';
	import SheetFooterActions from '#lib/components/sheets/sheet-footer-actions.svelte';
	import { Input } from '#lib/components/ui/input/index.js';
	import * as ResponsiveDialog from '#lib/components/ui/responsive-dialog/index.js';
	import * as Select from '#lib/components/ui/select/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { UserGroupLink, UserGroupLinkKind } from '#lib/types/usergroup.js';
	import { applyApiFieldError } from '#lib/utils/api.js';
	import { assignmentSourceBadge } from '#lib/utils/assignment-source.js';
	import { createForm, preventDefault } from '#lib/utils/settings.svelte.js';

	import type { UserGroupEditor } from '../use-user-group-editor.svelte.js';
	let { editor }: { editor: UserGroupEditor } = $props();

	// Each sheet session is keyed, so the group it opened with stays fixed.
	const group = untrack(() => editor.editing);
	const isSubmitting = $derived(editor.isSaving);
	const linkKinds: UserGroupLinkKind[] = ['oidc', 'ldap'];
	const LINK_VALUE_MAX_LENGTH = 255;

	// Matches the backend's limits on user groups.
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

	let links = $state<UserGroupLink[]>(group?.links.map((link) => ({ ...link })) ?? []);
	let linksError = $state<string | null>(null);

	// Blank rows are dropped rather than rejected.
	function validLinks(): UserGroupLink[] | null {
		const trimmed = links.map((link) => ({ kind: link.kind, value: link.value.trim() })).filter((link) => link.value);
		if (trimmed.some((link) => link.value.length > LINK_VALUE_MAX_LENGTH)) {
			linksError = m.common_max_length({ field: m.value(), maxLength: LINK_VALUE_MAX_LENGTH });
			return null;
		}
		linksError = null;
		return trimmed;
	}

	async function handleSubmit() {
		const data = groupForm.validate();
		const nextLinks = validLinks();
		if (!data || !nextLinks) return;
		applyApiFieldError(groupInputs, await editor.submit({ ...data, links: nextLinks }));
	}
</script>

<ResponsiveDialog.Root
	bind:open={() => editor.sheetOpen, (open) => (editor.sheetOpen = open)}
	variant="sheet"
	title={group ? m.edit_resource({ resource: m.user_group() }) : m.common_create_button({ resource: m.user_group() })}
	description={m.user_groups_description()}
	contentClass="sm:max-w-140"
>
	<form id="user-group-form" onsubmit={preventDefault(handleSubmit)} class="grid gap-4 py-6">
		<FormInput label={m.common_name()} placeholder={m.user_group_name_placeholder()} bind:input={groupInputs.name} />
		<FormInput label={m.common_description()} type="textarea" bind:input={groupInputs.description} />

		<div class="grid gap-2">
			<p class="text-sm font-medium">{m.linked_groups()}</p>
			<p class="text-xs text-muted-foreground">{m.linked_groups_description()}</p>
			<RowListEditor
				bind:rows={links}
				createRow={(): UserGroupLink => ({ kind: 'oidc', value: '' })}
				addLabel={m.add_linked_group()}
				disabled={isSubmitting}
			>
				{#snippet children(link)}
					<Select.Root
						type="single"
						value={link.kind}
						disabled={isSubmitting}
						onValueChange={(value) => (link.kind = value === 'ldap' ? 'ldap' : 'oidc')}
					>
						<Select.Trigger class="w-full sm:w-28" aria-label={m.common_type()}>
							<span>{assignmentSourceBadge(link.kind).label}</span>
						</Select.Trigger>
						<Select.Content>
							{#each linkKinds as kind (kind)}
								<Select.Item value={kind} label={assignmentSourceBadge(kind).label}
									>{assignmentSourceBadge(kind).label}</Select.Item
								>
							{/each}
						</Select.Content>
					</Select.Root>
					<Input
						type="text"
						class="flex-1"
						aria-label={m.value()}
						placeholder={m.linked_group_value_placeholder()}
						bind:value={link.value}
						disabled={isSubmitting}
					/>
				{/snippet}
			</RowListEditor>
			{#if linksError}
				<p class="text-xs font-medium text-destructive">{linksError}</p>
			{/if}
		</div>
	</form>

	{#snippet footer()}
		<SheetFooterActions
			bind:open={() => editor.sheetOpen, (open) => (editor.sheetOpen = open)}
			cancelDisabled={isSubmitting}
			submitAction={group ? 'save' : 'create'}
			submitLabel={group ? m.common_save() : m.common_create()}
			submitDisabled={isSubmitting}
			submitLoading={isSubmitting}
			submitForm="user-group-form"
		/>
	{/snippet}
</ResponsiveDialog.Root>
