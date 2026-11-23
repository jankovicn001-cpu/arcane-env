import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { toast } from 'svelte-sonner';

import { openConfirmDialog } from '#lib/components/confirm-dialog/index.js';
import { m } from '#lib/paraglide/messages.js';
import { queryKeys } from '#lib/query/query-keys.js';
import { userGroupService } from '#lib/services/user-group-service.js';
import type { UserGroupCreate, UserGroupUpdate } from '#lib/types/ui/usergroup.js';
import type { UserGroup } from '#lib/types/usergroup.js';
import { extractApiErrorMessage } from '#lib/utils/api.js';
import { isGlobalAdmin } from '#lib/utils/auth.js';
import { changedFields } from '#lib/utils/payload.js';
import { tryCatch } from '#lib/utils/try-catch.js';

type EditorOptions = {
	onDeleted?: (group: UserGroup) => unknown;
};

type SaveRequest = { id: string; dto: UserGroupUpdate } | { id?: undefined; dto: UserGroupCreate };

/** Create, edit, and delete for user groups, shared by the list and detail pages. */
export function useUserGroupEditor({ onDeleted }: EditorOptions = {}) {
	const queryClient = useQueryClient();

	let sheetOpen = $state(false);
	let sheetSession = $state(0);
	let editing = $state<UserGroup | null>(null);
	let deletingId = $state<string | null>(null);

	// Users carry their groups, and members gain or lose access with links and deletes.
	async function refreshAfterChange() {
		await Promise.all([
			queryClient.invalidateQueries({ queryKey: queryKeys.userGroups.all }),
			queryClient.invalidateQueries({ queryKey: queryKeys.users.all }),
			queryClient.invalidateQueries({ queryKey: queryKeys.environmentGroups.all })
		]);
	}

	const saveMutation = createMutation(() => ({
		mutationFn: (request: SaveRequest) =>
			request.id === undefined ? userGroupService.create(request.dto) : userGroupService.update(request.id, request.dto),
		onSuccess: async (saved, { id }) => {
			sheetOpen = false;
			if (id) {
				toast.success(m.common_update_success({ resource: saved.name }));
			} else {
				toast.success(m.common_create_success({ resource: saved.name }));
			}
			await refreshAfterChange();
		},
		onError: (error, { id }) => {
			let headline = m.common_create_failed({ resource: m.user_group() });
			if (id) headline = m.common_update_failed({ resource: m.user_group() });
			toast.error(headline, { description: extractApiErrorMessage(error) });
		}
	}));

	const deleteMutation = createMutation(() => ({
		mutationFn: (group: UserGroup) => userGroupService.delete(group.id),
		onMutate: (group) => {
			deletingId = group.id;
		},
		onSuccess: async (_, group) => {
			toast.success(m.common_delete_success({ resource: group.name }));
			await onDeleted?.(group);
			await refreshAfterChange();
		},
		onError: (error, group) => {
			toast.error(m.common_delete_failed({ resource: group.name }), { description: extractApiErrorMessage(error) });
		},
		onSettled: () => {
			deletingId = null;
		}
	}));

	return {
		// Every user group write is reserved for global admins on the server.
		get canManage() {
			return isGlobalAdmin();
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
		open(group: UserGroup | null) {
			editing = group;
			sheetSession += 1;
			sheetOpen = true;
		},
		async submit(dto: Required<UserGroupCreate>): Promise<Error | null> {
			const request: SaveRequest = editing ? { id: editing.id, dto: changedFields(editing, dto) } : { dto };
			const result = await tryCatch(saveMutation.mutateAsync(request));
			return result.error;
		},
		confirmDelete(group: UserGroup) {
			openConfirmDialog({
				title: m.common_delete_title({ resource: group.name }),
				message: m.delete_user_group_confirm({ name: group.name }),
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

export type UserGroupEditor = ReturnType<typeof useUserGroupEditor>;
