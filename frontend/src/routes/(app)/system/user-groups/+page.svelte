<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';

	import type { ActionButton } from '#lib/components/action-button-group/types.js';
	import * as Alert from '#lib/components/ui/alert/index.js';
	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import { AlertIcon, CommunityIcon } from '#lib/icons/index.js';
	import { SettingsPageLayout } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { userGroupListQuery } from '#lib/query/user-groups.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';

	import UserGroupSheet from './components/user-group-sheet.svelte';
	import UserGroupsTable from './components/user-groups-table.svelte';
	import { useUserGroupEditor } from './use-user-group-editor.svelte.js';

	const groupsQuery = createQuery(() => userGroupListQuery());
	const editor = useUserGroupEditor();

	const actionButtons: ActionButton[] = $derived.by(() => {
		if (!editor.canManage) return [];
		return [
			{
				id: 'create',
				action: 'create',
				placement: 'primary',
				label: m.common_create_button({ resource: m.user_group() }),
				onclick: () => editor.open(null)
			}
		];
	});
</script>

<SettingsPageLayout title={m.user_groups()} description={m.user_groups_description()} pageType="management" {actionButtons}>
	{#snippet mainContent()}
		{#if groupsQuery.isError}
			<Alert.Root variant="destructive">
				<AlertIcon class="size-4" />
				<Alert.Title>{m.common_load_failed({ resource: m.user_groups() })}</Alert.Title>
				<Alert.Description>{extractApiErrorMessage(groupsQuery.error)}</Alert.Description>
			</Alert.Root>
		{:else if groupsQuery.isPending}
			<div class="flex items-center justify-center py-8">
				<Spinner class="size-6" />
			</div>
		{:else if groupsQuery.data.length === 0}
			<div class="rounded-xl border border-dashed p-6 text-center text-sm text-muted-foreground">
				<CommunityIcon class="mx-auto mb-2 size-8 opacity-40" />
				{m.no_user_groups_yet()}
			</div>
		{:else}
			<UserGroupsTable groups={groupsQuery.data} {editor} />
		{/if}
	{/snippet}

	{#snippet additionalContent()}
		{#if editor.sheetSession > 0}
			{#key editor.sheetSession}
				<UserGroupSheet {editor} />
			{/key}
		{/if}
	{/snippet}
</SettingsPageLayout>
