import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { toast } from 'svelte-sonner';

import { openConfirmDialog } from '#lib/components/confirm-dialog/index.js';
import { m } from '#lib/paraglide/messages.js';
import { queryKeys } from '#lib/query/query-keys.js';
import { environmentGroupService } from '#lib/services/environment-group-service.js';
import { environmentStore } from '#lib/stores/environment.store.svelte.js';
import type { EnvironmentGroupRef } from '#lib/types/environment.js';
import { GLOBAL_SCOPE } from '#lib/types/role.js';
import type { EnvironmentGroupCreate, EnvironmentGroupUpdate } from '#lib/types/ui/environment.js';
import { extractApiErrorMessage } from '#lib/utils/api.js';
import { hasPermission } from '#lib/utils/auth.js';
import { changedFields } from '#lib/utils/payload.js';
import { tryCatch } from '#lib/utils/try-catch.js';

type EditorOptions = {
	onDeleted?: (group: EnvironmentGroupRef) => unknown;
};

type SaveRequest = { id: string; dto: EnvironmentGroupUpdate } | { id?: undefined; dto: EnvironmentGroupCreate };

/** Create, edit, and delete for environment groups, shared by every page that manages them. */
export function useEnvironmentGroupEditor({ onDeleted }: EditorOptions = {}) {
	const queryClient = useQueryClient();

	let sheetOpen = $state(false);
	let sheetSession = $state(0);
	let editing = $state<EnvironmentGroupRef | null>(null);
	let deletingId = $state<string | null>(null);

	// Agents carry their group, so the store is patched until the live stream catches up.
	async function refreshAfterChange(groupId: string, group: EnvironmentGroupRef | null) {
		await environmentStore.initialize(
			environmentStore.available.map((env) => {
				if (env.group?.id !== groupId) return env;
				if (!group) return { ...env, groupId: undefined, group: undefined };
				return { ...env, group: { id: group.id, name: group.name, description: group.description } };
			})
		);
		await Promise.all([
			queryClient.invalidateQueries({ queryKey: queryKeys.environmentGroups.all }),
			queryClient.invalidateQueries({ queryKey: queryKeys.environments.all })
		]);
	}

	const saveMutation = createMutation(() => ({
		mutationFn: (request: SaveRequest) =>
			request.id === undefined
				? environmentGroupService.create(request.dto)
				: environmentGroupService.update(request.id, request.dto),
		onSuccess: async (saved, { id }) => {
			sheetOpen = false;
			if (id) {
				toast.success(m.common_update_success({ resource: saved.name }));
			} else {
				toast.success(m.common_create_success({ resource: saved.name }));
			}
			await refreshAfterChange(saved.id, saved);
		},
		onError: (error, { id }) => {
			let headline = m.common_create_failed({ resource: m.environment() });
			if (id) headline = m.common_update_failed({ resource: m.environment() });
			toast.error(headline, { description: extractApiErrorMessage(error) });
		}
	}));

	const deleteMutation = createMutation(() => ({
		mutationFn: (group: EnvironmentGroupRef) => environmentGroupService.delete(group.id),
		onMutate: (group) => {
			deletingId = group.id;
		},
		onSuccess: async (_, group) => {
			toast.success(m.common_delete_success({ resource: group.name }));
			await refreshAfterChange(group.id, null);
			await onDeleted?.(group);
		},
		onError: (error, group) => {
			toast.error(m.common_delete_failed({ resource: group.name }), { description: extractApiErrorMessage(error) });
		},
		onSettled: () => {
			deletingId = null;
		}
	}));

	return {
		get canCreate() {
			return hasPermission('environments:create', GLOBAL_SCOPE);
		},
		get canUpdate() {
			return hasPermission('environments:update', GLOBAL_SCOPE);
		},
		get canDelete() {
			return hasPermission('environments:delete', GLOBAL_SCOPE);
		},
		get sheetOpen() {
			return sheetOpen;
		},
		set sheetOpen(open: boolean) {
			sheetOpen = open;
		},
		get sheetSession() {
			return sheetSession;
		},
		get editing() {
			return editing;
		},
		get deletingId() {
			return deletingId;
		},
		get isSaving() {
			return saveMutation.isPending;
		},
		/** Opens the sheet; null creates a group. */
		open(group: EnvironmentGroupRef | null) {
			editing = group;
			sheetSession += 1;
			sheetOpen = true;
		},
		async submit(dto: Required<EnvironmentGroupCreate>): Promise<Error | null> {
			const request: SaveRequest = editing ? { id: editing.id, dto: changedFields(editing, dto) } : { dto };
			const result = await tryCatch(saveMutation.mutateAsync(request));
			return result.error;
		},
		confirmDelete(group: EnvironmentGroupRef) {
			openConfirmDialog({
				title: m.common_delete_title({ resource: group.name }),
				message: m.delete_environment_confirm({ name: group.name }),
				confirm: {
					label: m.common_delete(),
					destructive: true,
					action: async () => {
						await tryCatch(deleteMutation.mutateAsync(group));
					}
				}
			});
		}
	};
}

export type EnvironmentGroupEditor = ReturnType<typeof useEnvironmentGroupEditor>;
