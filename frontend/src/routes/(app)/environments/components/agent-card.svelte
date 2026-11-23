<script lang="ts">
	import { resolve } from '$app/paths';

	import { Badge } from '#lib/components/ui/badge/index.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { AgentIcon, EnvironmentsIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { LOCAL_DOCKER_ENVIRONMENT_ID, type EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
	import type { ContainerStatusCounts } from '#lib/types/container.js';

	let {
		environment,
		groupId,
		online,
		counts
	}: {
		environment: EnvironmentRef;
		groupId: string;
		online: boolean;
		/** Container counts; undefined while loading or when the user cannot list containers. */
		counts?: ContainerStatusCounts;
	} = $props();

	const Icon = $derived(environment.id === LOCAL_DOCKER_ENVIRONMENT_ID ? EnvironmentsIcon : AgentIcon);
</script>

<a
	href={resolve('/(app)/environments/[groupId]/[environmentId]', { groupId, environmentId: environment.id })}
	class="rounded-xl focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
>
	<Card.Root interactive class="h-full">
		<Card.Content>
			<div class="flex flex-col gap-4">
				<div class="flex min-w-0 items-center gap-2">
					<Icon class="size-4 shrink-0 text-muted-foreground" />
					<span class="min-w-0 flex-1 truncate font-semibold">{environment.name}</span>
					<Badge variant={online ? 'green' : 'red'} size="sm">
						{online ? m.common_online() : m.common_offline()}
					</Badge>
				</div>
				<dl class="grid grid-cols-3 gap-2 text-sm">
					<div>
						<dt class="text-muted-foreground">{m.common_total()}</dt>
						<dd class="font-medium tabular-nums">{counts?.totalContainers ?? '—'}</dd>
					</div>
					<div>
						<dt class="text-muted-foreground">{m.common_running()}</dt>
						<dd class="font-medium tabular-nums">{counts?.runningContainers ?? '—'}</dd>
					</div>
					<div>
						<dt class="text-muted-foreground">{m.common_stopped()}</dt>
						<dd class="font-medium tabular-nums">{counts?.stoppedContainers ?? '—'}</dd>
					</div>
				</dl>
			</div>
		</Card.Content>
	</Card.Root>
</a>
