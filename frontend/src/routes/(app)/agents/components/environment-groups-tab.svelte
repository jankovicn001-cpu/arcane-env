<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';

	import { ArcaneButton } from '#lib/components/arcane-button/index.js';
	import EnvironmentGroupSheet from '#lib/components/sheets/environment-group-sheet.svelte';
	import * as Alert from '#lib/components/ui/alert/index.js';
	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { useEnvironmentGroupEditor } from '#lib/hooks/use-environment-group-editor.svelte.js';
	import { AddIcon, AlertIcon, EditIcon, EnvironmentsIcon, TrashIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { environmentGroupListQuery } from '#lib/query/environment-groups.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';

	const groupsQuery = createQuery(() => environmentGroupListQuery());
	const editor = useEnvironmentGroupEditor();
</script>

<section class="space-y-5">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h2 class="text-base font-semibold tracking-tight sm:text-lg">{m.environments()}</h2>
			<p class="mt-1 text-xs text-muted-foreground sm:text-sm">{m.group_agents_description()}</p>
		</div>
		{#if editor.canCreate}
			<ArcaneButton
				action="create"
				customLabel={m.common_create_button({ resource: m.environment() })}
				icon={AddIcon}
				class="shrink-0"
				onclick={() => editor.open(null)}
			/>
		{/if}
	</div>

	{#if groupsQuery.isError}
		<Alert.Root variant="destructive">
			<AlertIcon class="size-4" />
			<Alert.Title>{m.common_load_failed({ resource: m.environments() })}</Alert.Title>
			<Alert.Description>{extractApiErrorMessage(groupsQuery.error)}</Alert.Description>
		</Alert.Root>
	{:else}
		<div class="overflow-x-auto rounded-xl border bg-background/60">
			{#if groupsQuery.isPending}
				<div class="flex items-center justify-center py-8">
					<Spinner class="size-6" />
				</div>
			{:else if groupsQuery.data.length === 0}
				<div class="p-6 text-center text-sm text-muted-foreground">
					<EnvironmentsIcon class="mx-auto mb-2 size-8 opacity-40" />
					{m.no_environments_yet()}
				</div>
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{m.common_name()}</Table.Head>
							<Table.Head>{m.common_description()}</Table.Head>
							<Table.Head>{m.environments_title()}</Table.Head>
							<Table.Head><span class="sr-only">{m.common_actions()}</span></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each groupsQuery.data as group (group.id)}
							<Table.Row>
								<Table.Cell><span class="font-medium">{group.name}</span></Table.Cell>
								<Table.Cell><span class="text-muted-foreground">{group.description || '—'}</span></Table.Cell>
								<Table.Cell><span class="tabular-nums">{group.environmentCount}</span></Table.Cell>
								<Table.Cell>
									<div class="flex justify-end gap-1">
										{#if editor.canUpdate}
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
										{/if}
										{#if editor.canDelete}
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
</section>

{#if editor.sheetSession > 0}
	{#key editor.sheetSession}
		<EnvironmentGroupSheet {editor} />
	{/key}
{/if}
