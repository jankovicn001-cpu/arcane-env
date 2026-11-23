<script lang="ts">
	import { resolve } from '$app/paths';

	import * as Breadcrumb from '#lib/components/ui/breadcrumb/index.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		group,
		current
	}: {
		/** The environment group above the current page; omitted on the group's own page. */
		group?: { id: string; name: string };
		current: string;
	} = $props();
</script>

<Breadcrumb.Root>
	<Breadcrumb.List>
		<Breadcrumb.Item>
			<Breadcrumb.Link href={resolve('/(app)/environments')}>{m.environments()}</Breadcrumb.Link>
		</Breadcrumb.Item>
		<Breadcrumb.Separator />
		{#if group}
			<Breadcrumb.Item>
				<Breadcrumb.Link href={resolve('/(app)/environments/[groupId]', { groupId: group.id })}>{group.name}</Breadcrumb.Link>
			</Breadcrumb.Item>
			<Breadcrumb.Separator />
		{/if}
		<Breadcrumb.Item>
			<Breadcrumb.Page>{current}</Breadcrumb.Page>
		</Breadcrumb.Item>
	</Breadcrumb.List>
</Breadcrumb.Root>
