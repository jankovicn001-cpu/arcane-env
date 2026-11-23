<script lang="ts">
	import { ArcaneButton } from '#lib/components/arcane-button/index.js';
	import { EmptyState } from '#lib/components/states/index.js';
	import { AlertIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { formatRelativeTime } from '#lib/utils/formatting.js';

	import type { EnvironmentReachability } from '../environment-tabs';

	let {
		environment,
		status,
		error,
		retrying,
		onRetry
	}: {
		environment: EnvironmentRef;
		status: EnvironmentReachability;
		/** The failed request's error, when one was made. */
		error: unknown;
		retrying: boolean;
		onRetry: () => Promise<unknown>;
	} = $props();
</script>

<EmptyState
	icon={AlertIcon}
	title={status.online ? m.name_is_unreachable({ name: environment.name }) : m.name_is_offline({ name: environment.name })}
	description={status.online ? undefined : m.environment_offline_description()}
>
	<div class="flex flex-col items-center gap-3 text-sm">
		{#if !status.online && status.lastSeen}
			<p class="text-muted-foreground">{m.last_seen_time({ time: formatRelativeTime(status.lastSeen) })}</p>
		{/if}
		{#if error}
			<p class="text-destructive">{extractApiErrorMessage(error)}</p>
		{/if}
		<ArcaneButton action="refresh" customLabel={m.common_retry()} onclick={onRetry} loading={retrying} />
	</div>
</EmptyState>
