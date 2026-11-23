<script lang="ts">
	import { resolve } from '$app/paths';

	import { ArcaneButton } from '#lib/components/arcane-button/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { EditIcon, TrashIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { UserGroup } from '#lib/types/usergroup.js';
	import { assignmentSourceBadge } from '#lib/utils/assignment-source.js';

	import type { UserGroupEditor } from '../use-user-group-editor.svelte.js';

	let { groups, editor }: { groups: UserGroup[]; editor: UserGroupEditor } = $props();
</script>

<div class="overflow-x-auto rounded-xl border bg-background/60">
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head>{m.common_name()}</Table.Head>
				<Table.Head>{m.common_description()}</Table.Head>
				<Table.Head>{m.members()}</Table.Head>
				<Table.Head>{m.linked_groups()}</Table.Head>
				<Table.Head><span class="sr-only">{m.common_actions()}</span></Table.Head>
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#each groups as group (group.id)}
				<Table.Row>
					<Table.Cell>
						<!-- The detail page manages members and access, which only global admins may change. -->
						{#if editor.canManage}
							<a href={resolve('/(app)/system/user-groups/[id]', { id: group.id })} class="font-medium hover:underline">
								{group.name}
							</a>
						{:else}
							<span class="font-medium">{group.name}</span>
						{/if}
					</Table.Cell>
					<Table.Cell><span class="text-muted-foreground">{group.description || '—'}</span></Table.Cell>
					<Table.Cell><span class="tabular-nums">{group.memberCount}</span></Table.Cell>
					<Table.Cell>
						{#if group.links.length === 0}
							<span class="text-muted-foreground">—</span>
						{:else}
							<div class="flex flex-wrap gap-1">
								{#each group.links as link (`${link.kind}:${link.value}`)}
									{@const kind = assignmentSourceBadge(link.kind)}
									<Badge variant={kind.variant} size="sm">{kind.label}: {link.value}</Badge>
								{/each}
							</div>
						{/if}
					</Table.Cell>
					<Table.Cell>
						{#if editor.canManage}
							<div class="flex justify-end gap-1">
								<ArcaneButton
									action="base"
									tone="ghost"
									size="icon"
									icon={EditIcon}
									customLabel={m.common_edit()}
									showLabel={false}
									class="text-muted-foreground"
									onclick={() => editor.open(group)}
								/>
								<ArcaneButton
									action="remove"
									tone="ghost"
									size="icon"
									icon={TrashIcon}
									customLabel={m.common_delete()}
									showLabel={false}
									loading={editor.deletingId === group.id}
									disabled={editor.deletingId !== null}
									class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
									onclick={() => editor.confirmDelete(group)}
								/>
							</div>
						{/if}
					</Table.Cell>
				</Table.Row>
			{/each}
		</Table.Body>
	</Table.Root>
</div>
