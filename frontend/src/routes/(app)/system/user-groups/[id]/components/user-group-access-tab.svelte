<script lang="ts">
	import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';

	import { ArcaneButton } from '#lib/components/arcane-button/index.js';
	import { openConfirmDialog } from '#lib/components/confirm-dialog/index.js';
	import RoleScopeSelects from '#lib/components/sheets/role-scope-selects.svelte';
	import * as Alert from '#lib/components/ui/alert/index.js';
	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { AlertIcon, ShieldAlertIcon, TrashIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { userGroupAssignmentsQuery } from '#lib/query/user-groups.js';
	import { userGroupService } from '#lib/services/user-group-service.js';
	import type { Environment, EnvironmentGroupRef } from '#lib/types/environment.js';
	import type { Role } from '#lib/types/role.js';
	import type { UserGroupRoleAssignmentCreate } from '#lib/types/ui/usergroup.js';
	import type { UserGroupRoleAssignment } from '#lib/types/usergroup.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import {
		buildRoleScopeOptions,
		createRoleEnvironmentLabelers,
		GLOBAL_ENVIRONMENT_OPTION_ID,
		roleScopeFromOptionId,
		roleScopeLabel
	} from '#lib/utils/options.js';
	import { preventDefault } from '#lib/utils/settings.svelte.js';
	import { tryCatch } from '#lib/utils/try-catch.js';

	type Props = {
		groupId: string;
		roles: Role[];
		environments: Environment[];
		environmentGroups: EnvironmentGroupRef[];
	};

	let { groupId, roles, environments, environmentGroups }: Props = $props();

	const queryClient = useQueryClient();
	const assignmentsQuery = createQuery(() => userGroupAssignmentsQuery(groupId));

	const scopeOptions = $derived(buildRoleScopeOptions(environments, environmentGroups));
	const selectedLabel = $derived(createRoleEnvironmentLabelers(roles, scopeOptions, m.common_select_option()));

	let roleId = $state('');
	let scope = $state(GLOBAL_ENVIRONMENT_OPTION_ID);
	let revokingId = $state<string | null>(null);

	function scopeName(assignment: UserGroupRoleAssignment): string {
		return roleScopeLabel(assignment, environments, environmentGroups);
	}

	// Environment access lists show group grants, and users' permissions follow them.
	async function refreshAccess() {
		await Promise.all([
			queryClient.invalidateQueries({ queryKey: queryKeys.userGroups.assignments(groupId) }),
			queryClient.invalidateQueries({ queryKey: queryKeys.environmentGroups.all }),
			queryClient.invalidateQueries({ queryKey: queryKeys.users.all })
		]);
	}

	const grantMutation = createMutation(() => ({
		mutationFn: (dto: UserGroupRoleAssignmentCreate) => userGroupService.grant(groupId, dto),
		onSuccess: async (assignment) => {
			toast.success(m.role_granted_on_scope({ role: assignment.roleName, scope: scopeName(assignment) }));
			roleId = '';
			scope = GLOBAL_ENVIRONMENT_OPTION_ID;
			await refreshAccess();
		},
		onError: (error) => {
			toast.error(m.grant_access_failed(), { description: extractApiErrorMessage(error) });
		}
	}));

	const revokeMutation = createMutation(() => ({
		mutationFn: (assignment: UserGroupRoleAssignment) => userGroupService.revoke(groupId, assignment.id),
		onMutate: (assignment) => {
			revokingId = assignment.id;
		},
		onSuccess: async (_, assignment) => {
			toast.success(m.role_removed_from_scope({ role: assignment.roleName, scope: scopeName(assignment) }));
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
		if (!roleId) return;
		grantMutation.mutate({ roleId, ...roleScopeFromOptionId(scope) });
	}

	function confirmRevoke(assignment: UserGroupRoleAssignment) {
		openConfirmDialog({
			title: m.remove_access(),
			message: m.remove_role_on_scope_confirm({ role: assignment.roleName, scope: scopeName(assignment) }),
			confirm: {
				label: m.common_remove(),
				destructive: true,
				action: async () => {
					await tryCatch(revokeMutation.mutateAsync(assignment));
				}
			}
		});
	}
</script>

<div class="grid gap-6">
	<form onsubmit={preventDefault(handleGrant)} class="grid gap-4 rounded-xl border p-4">
		<div>
			<h3 class="text-sm font-semibold">{m.grant_access()}</h3>
			<p class="mt-1 text-xs text-muted-foreground">{m.user_group_access_description()}</p>
		</div>
		<RoleScopeSelects
			idPrefix="user-group-access"
			roleLabel={m.common_role()}
			scopeLabel={m.common_scope()}
			{roles}
			envOptions={scopeOptions}
			bind:roleValue={roleId}
			bind:environmentValue={scope}
			roleSelectedLabel={selectedLabel.role}
			envSelectedLabel={selectedLabel.environment}
			disabled={grantMutation.isPending}
			class="grid gap-4 sm:grid-cols-2"
		/>
		<ArcaneButton
			action="create"
			type="submit"
			customLabel={m.grant_access()}
			class="justify-self-end"
			loading={grantMutation.isPending}
			disabled={!roleId || grantMutation.isPending}
		/>
	</form>

	{#if assignmentsQuery.isError}
		<Alert.Root variant="destructive">
			<AlertIcon class="size-4" />
			<Alert.Title>{m.common_load_failed({ resource: m.access() })}</Alert.Title>
			<Alert.Description>{extractApiErrorMessage(assignmentsQuery.error)}</Alert.Description>
		</Alert.Root>
	{:else}
		<div class="overflow-x-auto rounded-xl border bg-background/60">
			{#if assignmentsQuery.isPending}
				<div class="flex items-center justify-center py-8">
					<Spinner class="size-6" />
				</div>
			{:else if assignmentsQuery.data.length === 0}
				<div class="p-6 text-center text-sm text-muted-foreground">
					<ShieldAlertIcon class="mx-auto mb-2 size-8 opacity-40" />
					{m.no_access_granted()}
				</div>
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{m.common_role()}</Table.Head>
							<Table.Head>{m.common_scope()}</Table.Head>
							<Table.Head><span class="sr-only">{m.common_actions()}</span></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each assignmentsQuery.data as assignment (assignment.id)}
							<Table.Row>
								<Table.Cell><span class="font-medium">{assignment.roleName}</span></Table.Cell>
								<Table.Cell>{scopeName(assignment)}</Table.Cell>
								<Table.Cell>
									<div class="flex justify-end">
										<ArcaneButton
											action="remove"
											tone="ghost"
											size="icon"
											icon={TrashIcon}
											customLabel={m.common_remove()}
											showLabel={false}
											loading={revokingId === assignment.id}
											disabled={revokingId !== null}
											class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
											onclick={() => confirmRevoke(assignment)}
										/>
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
