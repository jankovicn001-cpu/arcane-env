<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';

	import { environmentVersionInfoQuery } from '#lib/query/system.js';
	import type { Environment } from '#lib/types/environment.js';

	let { environment, isOnline }: { environment: Environment; isOnline: boolean } = $props();

	// Offline environments are never queried: remote version lookups are proxied
	// through the agent and would just time out.
	const versionQuery = createQuery(() => ({
		...environmentVersionInfoQuery(environment.id),
		enabled: isOnline,
		retry: false
	}));

	const info = $derived(versionQuery.data ?? null);
	const label = $derived(info?.displayVersion || info?.currentVersion || info?.currentTag || '');
</script>

{#if label}
	<span class="font-mono text-sm">{label}</span>
{:else}
	<span class="text-sm text-muted-foreground">—</span>
{/if}
