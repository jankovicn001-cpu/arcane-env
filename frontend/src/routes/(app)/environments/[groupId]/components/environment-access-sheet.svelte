<script lang="ts">
	import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';

	import { ArcaneButton } from '#lib/components/arcane-button/index.js';
	import { openConfirmDialog } from '#lib/components/confirm-dialog/index.js';
	import SelectWithLabel from '#lib/components/form/select-with-label.svelte';
	import UserPicker from '#lib/components/form/user-picker.svelte';
	import SegmentToggle from '#lib/components/segment-toggle.svelte';
	import * as Alert from '#lib/components/ui/alert/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import * as ResponsiveDialog from '#lib/components/ui/responsive-dialog/index.js';
	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { AlertIcon, CommunityIcon, TrashIcon, UsersIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { environmentGroupAccessQuery } from '#lib/query/environment-groups.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { roleAllQuery } from '#lib/query/roles.js';
	import { userGroupListQuery } from '#lib/query/user-groups.js';
	import { environmentGroupService } from '#lib/services/environment-group-service.js';
	import type { EnvironmentGroupRef } from '#lib/types/environment.js';
	import type { EnvironmentAccessEntry } from '#lib/types/role.js';
	import type { EnvironmentAccessCreate } from '#lib/types/ui/role.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { assignmentSourceBadge } from '#lib/utils/assignment-source.js';
	import { preventDefault } from '#lib/utils/settings.svelte.js';
	import { tryCatch } from '#lib/utils/try-catch.js';

	let { open = $bindable(false), group }: { open: boolean; group: EnvironmentGroupRef } = $props();

	type PrincipalType = EnvironmentAccessEntry['principalType'];

	const queryClient = useQueryClient();
	const accessQuery = createQuery(() => environmentGroupAccessQuery(group.id));
	const rolesQuery = createQuery(() => roleAllQuery());
	const userGroupsQuery = createQuery(() => userGroupListQuery());

	let principalType = $state<PrincipalType>('user');
	let userId = $state('');
	let userGroupId = $state('');
	let roleId = $state('');
	let revokingId = $state<string | null>(null);

	const principalOptions: { value: PrincipalType; label: string }[] = [
		{ value: 'user', label: m.common_user() },
		{ value: 'group', label: m.user_group() }
	];
	const roleOptions = $derived(
		(rolesQuery.data ?? []).map((role) => ({ value: role.id, label: role.name, description: role.description ?? undefined }))
	);
	const userGroupOptions = $derived(
		(userGroupsQuery.data ?? []).map((userGroup) => ({
			value: userGroup.id,
			label: userGroup.name,
			description: userGroup.description || undefined
		}))
	);
	const principalId = $derived(principalType === 'user' ? userId : userGroupId);

	function principalName(entry: EnvironmentAccessEntry): string {
		return entry.principalType === 'group' ? entry.userGroupName : entry.username;
	}

	// The users and user groups lists carry their assignments, so they are refreshed with the access list.
	async function refreshAccess() {
		await Promise.all([
			queryClient.invalidateQueries({ queryKey: queryKeys.environmentGroups.access(group.id) }),
			queryClient.invalidateQueries({ queryKey: queryKeys.users.all }),
			queryClient.invalidateQueries({ queryKey: queryKeys.userGroups.all })
		]);
	}

	const grantMutation = createMutation(() => ({
		mutationFn: (dto: EnvironmentAccessCreate) => environmentGroupService.grantAccess(group.id, dto),
		onSuccess: async (entry) => {
			toast.success(m.access_granted({ role: entry.roleName, user: principalName(entry) }));
			userId = '';
			userGroupId = '';
			await refreshAccess();
		},
		onError: (error) => {
			toast.error(m.grant_access_failed(), { description: extractApiErrorMessage(error) });
		}
	}));

	const revokeMutation = createMutation(() => ({
		mutationFn: (entry: EnvironmentAccessEntry) => environmentGroupService.revokeAccess(group.id, entry.id),
		onMutate: (entry) => {
			revokingId = entry.id;
		},
		onSuccess: async (_, entry) => {
			toast.success(m.access_removed({ role: entry.roleName, user: principalName(entry) }));
			await refreshAccess();
		},
		onError: (error) => {
			toast.error(m.remove_access_failed(), { description: extractApiErrorMessage(error) });
		},
		onSettled: () => {
			revokingId = null;
		}
	}));

	function handleGrant() {
		if (!principalId || !roleId) return;
		if (principalType === 'group') {
			grantMutation.mutate({ userGroupId, roleId });
		} else {
			grantMutation.mutate({ userId, roleId });
		}
	}

	function confirmRevoke(entry: EnvironmentAccessEntry) {
		openConfirmDialog({
			title: m.remove_access(),
			message: m.remove_access_confirm({ role: entry.roleName, user: principalName(entry), name: group.name }),
			confirm: {
				label: m.common_remove(),
				destructive: true,
				action: async () => {
					await tryCatch(revokeMutation.mutateAsync(entry));
				}
			}
		});
	}
</script>

<ResponsiveDialog.Root
	bind:open
	variant="sheet"
	title={m.access()}
	description={m.environment_access_description({ name: group.name })}
	contentClass="sm:max-w-160"
>
	<div class="grid gap-6 py-6">
		<form onsubmit={preventDefault(handleGrant)} class="grid gap-4 rounded-xl border p-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h3 class="text-sm font-semibold">{m.grant_access()}</h3>
				<SegmentToggle
					label={m.grant_to()}
					options={principalOptions}
					current={principalType}
					onSelect={(value) => (principalType = value === 'group' ? 'group' : 'user')}
				/>
			</div>
			<div class="grid gap-4 sm:grid-cols-2">
				{#if principalType === 'user'}
					<div class="space-y-2">
						<Label for="grant-access-user" class="mb-0">{m.common_user()}</Label>
						<UserPicker triggerId="grant-access-user" bind:value={userId} disabled={grantMutation.isPending} />
					</div>
				{:else}
					<SelectWithLabel
						id="grant-access-group"
						label={m.user_group()}
						bind:value={userGroupId}
						options={userGroupOptions}
						error={userGroupsQuery.error ? extractApiErrorMessage(userGroupsQuery.error) : undefined}
						disabled={grantMutation.isPending}
					/>
				{/if}
				<SelectWithLabel
					id="grant-access-role"
					label={m.common_role()}
					bind:value={roleId}
					options={roleOptions}
					error={rolesQuery.error ? extractApiErrorMessage(rolesQuery.error) : undefined}
					disabled={grantMutation.isPending}
				/>
			</div>
			<ArcaneButton
				action="create"
				type="submit"
				customLabel={m.grant_access()}
				class="justify-self-end"
				loading={grantMutation.isPending}
				disabled={!principalId || !roleId || grantMutation.isPending}
			/>
		</form>

		{#if accessQuery.isError}
			<Alert.Root variant="destructive">
				<AlertIcon class="size-4" />
				<Alert.Title>{m.common_load_failed({ resource: m.access() })}</Alert.Title>
				<Alert.Description>{extractApiErrorMessage(accessQuery.error)}</Alert.Description>
			</Alert.Root>
		{:else}
			<div class="overflow-x-auto rounded-xl border bg-background/60">
				{#if accessQuery.isPending}
					<div class="flex items-center justify-center py-8">
						<Spinner class="size-6" />
					</div>
				{:else if accessQuery.data.length === 0}
					<div class="p-6 text-center text-sm text-muted-foreground">
						<UsersIcon class="mx-auto mb-2 size-8 opacity-40" />
						{m.no_access_granted()}
					</div>
				{:else}
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{m.user_or_group()}</Table.Head>
								<Table.Head>{m.common_role()}</Table.Head>
								<Table.Head>{m.source()}</Table.Head>
								<Table.Head><span class="sr-only">{m.common_actions()}</span></Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each accessQuery.data as entry (entry.id)}
								{@const source = assignmentSourceBadge(entry.source)}
								<Table.Row>
									<Table.Cell>
										{#if entry.principalType === 'group'}
											<span class="flex items-center gap-2 font-medium">
												<CommunityIcon class="size-4 text-muted-foreground" aria-hidden="true" />
												{entry.userGroupName}
											</span>
										{:else}
											<div class="flex flex-col">
												<span class="font-medium">{entry.username}</span>
												{#if entry.displayName}
													<span class="text-xs text-muted-foreground">{entry.displayName}</span>
												{/if}
											</div>
										{/if}
									</Table.Cell>
									<Table.Cell>{entry.roleName}</Table.Cell>
									<Table.Cell><Badge variant={source.variant} size="sm">{source.label}</Badge></Table.Cell>
									<Table.Cell>
										<div class="flex justify-end">
											{#if entry.source === 'manual'}
												<ArcaneButton
													action="remove"
													tone="ghost"
													size="icon"
													icon={TrashIcon}
													customLabel={m.common_remove()}
													showLabel={false}
													loading={revokingId === entry.id}
													disabled={revokingId !== null}
													class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
													onclick={() => confirmRevoke(entry)}
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
</ResponsiveDialog.Root>
