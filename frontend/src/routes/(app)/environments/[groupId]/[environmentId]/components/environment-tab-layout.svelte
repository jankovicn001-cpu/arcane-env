<script lang="ts">
	import type { Snippet } from 'svelte';

	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import { ResourcePageLayout, type ActionButton, type StatCardConfig } from '#lib/layouts/index.js';
	import type { EnvironmentRef } from '#lib/stores/environment.store.svelte.js';

	import type { EnvironmentReachability } from '../environment-tabs';
	import EnvironmentUnavailable from './environment-unavailable.svelte';

	let {
		environment,
		status,
		subtitle,
		actionButtons,
		statCards,
		query,
		tabs,
		children,
		additionalContent
	}: {
		environment: EnvironmentRef;
		status: EnvironmentReachability;
		subtitle: string;
		actionButtons: ActionButton[];
		statCards: StatCardConfig[];
		/** The tab's list query; its rows render once the environment answered. */
		query: { data: unknown; error: unknown; isFetching: boolean; refetch: () => Promise<unknown> };
		tabs: Snippet;
		children: Snippet;
		additionalContent?: Snippet;
	} = $props();

	// An offline environment is only asked on Retry; a successful retry shows its rows until its status changes.
	let retried = $state(false);
	const reached = $derived(status.online || (retried && !query.isFetching && !query.error));
	const unavailable = $derived(!reached || (query.data === undefined && Boolean(query.error)));
	const ready = $derived(!unavailable && query.data !== undefined);

	async function retry() {
		retried = true;
		await query.refetch();
	}
</script>

<ResourcePageLayout
	title={environment.name}
	{subtitle}
	actionButtons={ready ? actionButtons : []}
	statCards={ready ? statCards : []}
	{additionalContent}
>
	{#snippet mainContent()}
		{@render tabs()}
		{#if unavailable}
			<EnvironmentUnavailable {environment} {status} error={query.error} retrying={query.isFetching} onRetry={retry} />
		{:else if ready}
			{@render children()}
		{:else}
			<div class="flex justify-center py-12">
				<Spinner class="size-6" />
			</div>
		{/if}
	{/snippet}
</ResourcePageLayout>
