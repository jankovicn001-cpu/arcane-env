<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createQuery } from '@tanstack/svelte-query';

	import type { ActionButton } from '#lib/components/action-button-group/types.js';
	import { TabBar } from '#lib/components/tab-bar/index.js';
	import * as Tabs from '#lib/components/ui/tabs/index.js';
	import { useUrlTab } from '#lib/hooks/use-url-tab.svelte.js';
	import { ShieldAlertIcon, UsersIcon } from '#lib/icons/index.js';
	import { SettingsPageLayout } from '#lib/layouts/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { userGroupDetailQuery } from '#lib/query/user-groups.js';

	import UserGroupSheet from '../components/user-group-sheet.svelte';
	import { useUserGroupEditor } from '../use-user-group-editor.svelte.js';
	import UserGroupAccessTab from './components/user-group-access-tab.svelte';
	import UserGroupMembersTab from './components/user-group-members-tab.svelte';

	let { data } = $props();

	const groupQuery = createQuery(() => userGroupDetailQuery(data.group.id));
	const group = $derived(groupQuery.data ?? data.group);
	const editor = useUserGroupEditor({ onDeleted: () => goto(resolve('/(app)/system/user-groups')) });

	const urlTab = useUrlTab({
		tabs: () => [
			{ value: 'members', label: m.members(), icon: UsersIcon },
			{ value: 'access', label: m.access(), icon: ShieldAlertIcon }
		],
		defaultTab: () => 'members'
	});

	const actionButtons: ActionButton[] = $derived([
		{ id: 'edit', action: 'edit', label: m.common_edit(), onclick: () => editor.open(group) },
		{
			id: 'delete',
			action: 'remove',
			group: 'danger',
			destructive: true,
			label: m.common_delete(),
			onclick: () => editor.confirmDelete(group),
			loading: editor.deletingId === group.id,
			disabled: editor.deletingId !== null
		}
	]);
</script>

<SettingsPageLayout title={group.name} description={group.description || undefined} pageType="management" {actionButtons}>
	{#snippet mainContent()}
		<Tabs.Root value={urlTab.value}>
			<div class="pb-6">
				<TabBar items={urlTab.items} onValueChange={urlTab.select} />
			</div>

			<Tabs.Content value="members">
				<UserGroupMembersTab groupId={group.id} />
			</Tabs.Content>

			<Tabs.Content value="access">
				<UserGroupAccessTab
					groupId={group.id}
					roles={data.roles}
					environments={data.environments}
					environmentGroups={data.environmentGroups}
				/>
			</Tabs.Content>
		</Tabs.Root>
	{/snippet}

	{#snippet additionalContent()}
		{#if editor.sheetSession > 0}
			{#key editor.sheetSession}
				<UserGroupSheet {editor} />
			{/key}
		{/if}
	{/snippet}
</SettingsPageLayout>
