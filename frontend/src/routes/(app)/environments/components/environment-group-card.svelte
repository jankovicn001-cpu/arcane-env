<script lang="ts">
	import { resolve } from '$app/paths';

	import { Badge } from '#lib/components/ui/badge/index.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { EnvironmentsIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { EnvironmentGroupBucket } from '#lib/utils/environments.js';

	let { group, online }: { group: EnvironmentGroupBucket; /** How many of the group's agents are online. */ online: number } =
		$props();

	const offline = $derived(group.environments.length - online);
</script>

<a
	href={resolve('/(app)/environments/[groupId]', { groupId: group.id })}
	class="rounded-xl focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
>
	<Card.Root interactive class="h-full">
		<Card.Content>
			<div class="flex flex-col gap-4">
				<div class="flex min-w-0 flex-col gap-1">
					<div class="flex min-w-0 items-center gap-2">
						<EnvironmentsIcon class="size-4 shrink-0 text-muted-foreground" />
						<span class="min-w-0 flex-1 truncate font-semibold">{group.name}</span>
					</div>
					{#if group.description}
						<p class="line-clamp-2 text-sm text-muted-foreground">{group.description}</p>
					{/if}
				</div>
				<div class="flex flex-wrap items-center gap-2 text-sm">
					<span class="font-medium tabular-nums">{m.count_agents({ count: group.environments.length })}</span>
					<Badge variant={online > 0 ? 'green' : 'gray'} size="sm">{m.count_online({ count: online })}</Badge>
					<Badge variant={offline > 0 ? 'red' : 'gray'} size="sm">{m.count_offline({ count: offline })}</Badge>
				</div>
			</div>
		</Card.Content>
	</Card.Root>
</a>
