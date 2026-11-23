<script lang="ts">
	import DockerTab from '#lib/components/settings/environment/docker-tab.svelte';
	import EnvironmentSettingsPage from '#lib/components/settings/environment/environment-settings-page.svelte';
	import { m } from '#lib/paraglide/messages.js';
</script>

<EnvironmentSettingsPage
	title={m.docker_title()}
	description={m.settings_category_docker_description()}
	keys={[
		'defaultShell',
		'defaultDeployPullPolicy',
		'toolsImageRegistry',
		'baseServerUrl',
		'autoInjectEnv',
		'pruneContainerMode',
		'pruneContainerUntil',
		'pruneImageMode',
		'pruneImageUntil',
		'pruneVolumeMode',
		'pruneNetworkMode',
		'pruneNetworkUntil',
		'pruneBuildCacheMode',
		'pruneBuildCacheUntil'
	]}
>
	{#snippet content({ formInputs, mixed })}
		<DockerTab {formInputs} {mixed} />
	{/snippet}
</EnvironmentSettingsPage>
