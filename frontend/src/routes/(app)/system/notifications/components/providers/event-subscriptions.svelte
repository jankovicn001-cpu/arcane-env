<script lang="ts">
	import SettingsRow from '#lib/components/settings/settings-row.svelte';
	import { Switch } from '#lib/components/ui/switch/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		providerId: string;
		eventVulnerabilityFound: boolean;
		eventPruneReport: boolean;
		eventAutoHeal: boolean;
		disabled?: boolean;
	}

	let {
		providerId,
		eventVulnerabilityFound = $bindable(),
		eventPruneReport = $bindable(),
		eventAutoHeal = $bindable(),
		disabled = false
	}: Props = $props();
</script>

<!-- Rendered inside the provider section, so each event is its own row. -->
<SettingsRow
	for="{providerId}-event-prune-report"
	label={m.notifications_event_prune_report_label()}
	description={m.notifications_event_prune_report_description()}
	layout="switch"
>
	{#snippet children({ describedBy })}
		<Switch id="{providerId}-event-prune-report" bind:checked={eventPruneReport} {disabled} aria-describedby={describedBy} />
	{/snippet}
</SettingsRow>
<SettingsRow
	for="{providerId}-event-vulnerability-found"
	label={m.notifications_event_vulnerability_found_label()}
	description={m.notifications_event_vulnerability_found_description()}
	layout="switch"
>
	{#snippet children({ describedBy })}
		<Switch
			id="{providerId}-event-vulnerability-found"
			bind:checked={eventVulnerabilityFound}
			{disabled}
			aria-describedby={describedBy}
		/>
	{/snippet}
</SettingsRow>
<SettingsRow
	for="{providerId}-event-auto-heal"
	label={m.notifications_event_auto_heal_label()}
	description={m.notifications_event_auto_heal_description()}
	layout="switch"
>
	{#snippet children({ describedBy })}
		<Switch id="{providerId}-event-auto-heal" bind:checked={eventAutoHeal} {disabled} aria-describedby={describedBy} />
	{/snippet}
</SettingsRow>
