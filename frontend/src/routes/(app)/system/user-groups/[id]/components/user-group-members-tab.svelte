<script lang="ts">
	import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';

	import { ArcaneButton } from '#lib/components/arcane-button/index.js';
	import { openConfirmDialog } from '#lib/components/confirm-dialog/index.js';
	import UserPicker from '#lib/components/form/user-picker.svelte';
	import * as Alert from '#lib/components/ui/alert/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { AlertIcon, TrashIcon, UsersIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { userGroupMembersQuery } from '#lib/query/user-groups.js';
	import { userGroupService } from '#lib/services/user-group-service.js';
	import type { UserGroupMember } from '#lib/types/usergroup.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { assignmentSourceBadge } from '#lib/utils/assignment-source.js';
	import { preventDefault } from '#lib/utils/settings.svelte.js';
	import { tryCatch } from '#lib/utils/try-catch.js';

	let { groupId }: { groupId: string } = $props();

	const queryClient = useQueryClient();
	const membersQuery = createQuery(() => userGroupMembersQuery(groupId));

	let userId = $state('');
	let removingId = $state<string | null>(null);

	// Member counts and each user's groups change with membership.
	async function refreshMembers() {
		await Promise.all([
			queryClient.invalidateQueries({ queryKey: queryKeys.userGroups.all }),
			queryClient.invalidateQueries({ queryKey: queryKeys.users.all })
		]);
	}

	const addMutation = createMutation(() => ({
		mutationFn: (id: string) => userGroupService.addMember(groupId, { userId: id }),
		onSuccess: async (member) => {
			toast.success(m.member_added({ user: member.username }));
			userId = '';
			await refreshMembers();
		},
		onError: (error) => {
			toast.error(m.add_member_failed(), { description: extractApiErrorMessage(error) });
		}
	}));

	const removeMutation = createMutation(() => ({
		mutationFn: (member: UserGroupMember) => userGroupService.removeMember(groupId, member.id),
		onMutate: (member) => {
			removingId = member.id;
		},
		onSuccess: async (_, member) => {
			toast.success(m.member_removed({ user: member.username }));
			await refreshMembers();
		},
		onError: (error) => {
			toast.error(m.remove_member_failed(), { description: extractApiErrorMessage(error) });
		},
		onSettled: () => {
			removingId = null;
		}
	}));

	function handleAdd() {
		if (!userId) return;
		addMutation.mutate(userId);
	}

	function confirmRemove(member: UserGroupMember) {
		openConfirmDialog({
			title: m.remove_member(),
			message: m.remove_member_confirm({ user: member.username }),
			confirm: {
				label: m.common_remove(),
				destructive: true,
				action: async () => {
					await tryCatch(removeMutation.mutateAsync(member));
				}
			}
		});
	}
</script>

<div class="grid gap-6">
	<form onsubmit={preventDefault(handleAdd)} class="grid gap-4 rounded-xl border p-4">
		<div class="space-y-2">
			<Label for="add-member-user" class="mb-0">{m.common_user()}</Label>
			<UserPicker triggerId="add-member-user" bind:value={userId} disabled={addMutation.isPending} />
		</div>
		<ArcaneButton
			action="create"
			type="submit"
			customLabel={m.add_member()}
			class="justify-self-end"
			loading={addMutation.isPending}
			disabled={!userId || addMutation.isPending}
		/>
	</form>

	{#if membersQuery.isError}
		<Alert.Root variant="destructive">
			<AlertIcon class="size-4" />
			<Alert.Title>{m.common_load_failed({ resource: m.members() })}</Alert.Title>
			<Alert.Description>{extractApiErrorMessage(membersQuery.error)}</Alert.Description>
		</Alert.Root>
	{:else}
		<div class="overflow-x-auto rounded-xl border bg-background/60">
			{#if membersQuery.isPending}
				<div class="flex items-center justify-center py-8">
					<Spinner class="size-6" />
				</div>
			{:else if membersQuery.data.length === 0}
				<div class="p-6 text-center text-sm text-muted-foreground">
					<UsersIcon class="mx-auto mb-2 size-8 opacity-40" />
					{m.no_members_yet()}
				</div>
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{m.common_user()}</Table.Head>
							<Table.Head>{m.source()}</Table.Head>
							<Table.Head><span class="sr-only">{m.common_actions()}</span></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each membersQuery.data as member (member.id)}
							{@const source = assignmentSourceBadge(member.source)}
							<Table.Row>
								<Table.Cell>
									<div class="flex flex-col">
										<span class="font-medium">{member.username}</span>
										{#if member.displayName}
											<span class="text-xs text-muted-foreground">{member.displayName}</span>
										{/if}
									</div>
								</Table.Cell>
								<Table.Cell><Badge variant={source.variant} size="sm">{source.label}</Badge></Table.Cell>
								<Table.Cell>
									<!-- SSO and LDAP members follow the login or sync, so only manual ones are removable. -->
									<div class="flex justify-end">
										{#if member.source === 'manual'}
											<ArcaneButton
												action="remove"
												tone="ghost"
												size="icon"
												icon={TrashIcon}
												customLabel={m.common_remove()}
												showLabel={false}
												loading={removingId === member.id}
												disabled={removingId !== null}
												class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
												onclick={() => confirmRemove(member)}
											/>
										{/if}
									</div>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		</div>
	{/if}
</div>
