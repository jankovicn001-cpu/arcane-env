<script lang="ts">
	import { TabBar } from '#lib/components/tab-bar/index.js';
	import * as Tabs from '#lib/components/ui/tabs/index.js';
	import { useUrlTab } from '#lib/hooks/use-url-tab.svelte.js';
	import { environmentStore } from '#lib/stores/environment.store.svelte.js';
	import { isEnvironmentOnline } from '#lib/utils/docker.js';

	import EnvironmentsBreadcrumb from '../../components/environments-breadcrumb.svelte';
	import EnvironmentContainers from './components/environment-containers.svelte';
	import EnvironmentImages from './components/environment-images.svelte';
	import EnvironmentNetworks from './components/environment-networks.svelte';
	import EnvironmentProjects from './components/environment-projects.svelte';
	import EnvironmentVolumes from './components/environment-volumes.svelte';
	import { DEFAULT_ENVIRONMENT_TAB, environmentTabItems, type EnvironmentReachability } from './environment-tabs';

	let { data } = $props();

	// The live record, so the page follows the environment going offline and coming back.
	const live = $derived(environmentStore.available.find((env) => env.id === data.environment.id));
	const status: EnvironmentReachability = $derived({
		online: live !== undefined && isEnvironmentOnline(live),
		lastSeen: live?.lastSeen ?? live?.lastHeartbeat
	});

	const urlTab = useUrlTab({
		tabs: () => environmentTabItems(data.environment.id),
		defaultTab: () => DEFAULT_ENVIRONMENT_TAB
	});
</script>

{#snippet tabBar()}
	<Tabs.Root value={urlTab.value}>
		<div class="pb-6">
			<TabBar items={urlTab.items} onValueChange={urlTab.select} />
		</div>
	</Tabs.Root>
{/snippet}

<div class="space-y-3">
	<EnvironmentsBreadcrumb group={data.group} current={data.environment.name} />
	<!-- Fresh tab state per environment and reachability, so selection, paging and retries never leak across. -->
	{#key `${data.environment.id}:${status.online}`}
		{#if urlTab.value === 'containers'}
			<EnvironmentContainers environment={data.environment} {status} tabs={tabBar} />
		{:else if urlTab.value === 'projects'}
			<EnvironmentProjects environment={data.environment} {status} tabs={tabBar} />
		{:else if urlTab.value === 'images'}
			<EnvironmentImages environment={data.environment} {status} tabs={tabBar} />
		{:else if urlTab.value === 'volumes'}
			<EnvironmentVolumes environment={data.environment} {status} tabs={tabBar} />
		{:else}
			<EnvironmentNetworks environment={data.environment} {status} tabs={tabBar} />
		{/if}
	{/key}
</div>
