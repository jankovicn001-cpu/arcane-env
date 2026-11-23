<script lang="ts">
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';

	import JobScheduleDialog from '#lib/components/job-card/job-schedule-dialog.svelte';
	import MixedValuesPopover from '#lib/components/settings/mixed-values-popover.svelte';
	import SettingsRow from '#lib/components/settings/settings-row.svelte';
	import SettingsSection from '#lib/components/settings/settings-section.svelte';
	import { Checkbox } from '#lib/components/ui/checkbox/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import * as ScrollArea from '#lib/components/ui/scroll-area/index.js';
	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import { Switch } from '#lib/components/ui/switch/index.js';
	import { ClockIcon, EditIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { containerListQuery } from '#lib/query/containers.js';
	import { jobAutomationsQuery, jobSchedulesQuery } from '#lib/query/jobs.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { jobScheduleService } from '#lib/services/job-schedule-service.js';
	import { featureStore } from '#lib/stores/features.store.svelte.js';
	import type { ContainerSummary } from '#lib/types/container.js';
	import type { JobScheduleConfig } from '#lib/types/jobschedule.js';
	import type { MixedSettingValue } from '#lib/types/ui/settings.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { hasPermission } from '#lib/utils/auth.js';
	import { listScopes, settleAcrossEnvironments } from '#lib/utils/environments.js';

	import type { AutomationsTabProps } from './tab-props';

	type ScheduleKey = keyof Pick<JobScheduleConfig, 'scheduledPruneInterval' | 'vulnerabilityScanInterval' | 'autoHealInterval'>;

	let { formInputs = $bindable(), mixed, lock, environments }: AutomationsTabProps = $props();

	const queryClient = useQueryClient();
	const vulnerabilityManagementEnabled = $derived(
		environments.some((environment) => featureStore.isEnabled('vulnerabilityManagement', environment.id))
	);

	const scheduleQuery = createQuery(() => jobSchedulesQuery(environments));
	const schedulesByEnvironment = $derived(
		new Map((scheduleQuery.data?.byEnvironment ?? []).map(({ environment, data }) => [environment.id, data]))
	);

	const jobListEnvironments = $derived(environments.filter((environment) => hasPermission('jobs:list', environment.id)));
	const jobListQuery = createQuery(() => ({
		...jobAutomationsQuery(jobListEnvironments),
		enabled: jobListEnvironments.length > 0
	}));
	const scheduleTimezone = $derived(
		[...new Set((jobListQuery.data?.byEnvironment ?? []).flatMap(({ data }) => (data.timezone ? [data.timezone] : [])))].join(
			', '
		)
	);

	const canEditSchedules = $derived(environments.every((environment) => hasPermission('jobs:update', environment.id)));
	const containerEnvironments = $derived(environments.filter((environment) => hasPermission('containers:list', environment.id)));

	const containersQuery = createQuery(() => ({
		...containerListQuery(containerEnvironments.map((environment) => environment.id).join(','), containerEnvironments, {
			pagination: { page: 1, limit: 100 }
		}),
		enabled: containerEnvironments.length > 0 && !!formInputs.autoHealEnabled.value
	}));
	const containers = $derived([
		...new Map((containersQuery.data?.data ?? []).map((container) => [containerName(container), container])).values()
	]);

	let autoHealSearchTerm = $state('');
	let scheduleTarget = $state<{ key: ScheduleKey; name: string; value?: string } | null>(null);
	let scheduleOpen = $state(false);

	const autoHealExcluded = $derived(
		new Set(
			(formInputs.autoHealExcludedContainers.value || '')
				.split(',')
				.map((entry) => entry.trim())
				.filter(Boolean)
		)
	);

	function containerName(container: ContainerSummary): string {
		return container.names[0] || container.id.substring(0, 12);
	}

	function toggleExcluded(current: ReadonlySet<string>, name: string): string {
		return (current.has(name) ? [...current].filter((entry) => entry !== name) : [...current, name]).join(',');
	}

	function scheduleValues(key: ScheduleKey): string[] {
		return [...new Set(environments.flatMap((environment) => schedulesByEnvironment.get(environment.id)?.[key] ?? []))];
	}

	function mixedSchedules(key: ScheduleKey, name: string): MixedSettingValue[] {
		const groups = Map.groupBy(
			environments.flatMap((environment) => {
				const value = schedulesByEnvironment.get(environment.id)?.[key];
				return value ? [{ value, name: environment.name }] : [];
			}),
			(entry) => entry.value
		);
		return [...groups].map(([value, entries]) => ({
			label: value,
			environments: entries.map((entry) => entry.name),
			apply: () => openSchedule(key, name, value)
		}));
	}

	function openSchedule(key: ScheduleKey, name: string, value?: string) {
		scheduleTarget = { key, name, value };
		scheduleOpen = true;
	}

	async function saveSchedule(update: Record<string, string>) {
		await settleAcrossEnvironments(environments, (environmentId) => jobScheduleService.updateJobSchedules(update, environmentId));
		await Promise.all(
			listScopes(environments.map((environment) => environment.id)).map((scope) =>
				queryClient.invalidateQueries({ queryKey: queryKeys.jobs.list(scope) })
			)
		);
	}
</script>

{#snippet scheduleControl(key: ScheduleKey, name: string)}
	{@const values = scheduleValues(key)}
	{#if values.length > 1}
		<span class="mt-1.5 inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<ClockIcon class="size-3.5" />
			{m.jobs_schedule()}:
			<MixedValuesPopover
				values={mixedSchedules(key, name)}
				class="rounded-sm font-mono transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
			>
				{m.mixed()}
			</MixedValuesPopover>
			{#if canEditSchedules}
				<button
					type="button"
					title={m.jobs_edit_schedule()}
					class="rounded-sm transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
					onclick={() => openSchedule(key, name)}
				>
					<EditIcon class="size-3" />
				</button>
			{/if}
		</span>
	{:else if values.length > 0}
		<button
			type="button"
			class="mt-1.5 inline-flex items-center gap-1.5 rounded-sm text-xs text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
			disabled={!canEditSchedules}
			onclick={() => openSchedule(key, name)}
		>
			<ClockIcon class="size-3.5" />
			{m.jobs_schedule()}:
			<span class="font-mono">{values[0]}</span>
			{#if canEditSchedules}
				<EditIcon class="size-3" />
			{/if}
		</button>
	{/if}
{/snippet}

{#snippet autoHealOptions()}
	<SettingsRow
		for="auto-heal-max-restarts"
		label={m.auto_heal_max_restarts_label()}
		description={m.auto_heal_max_restarts_description()}
		error={formInputs.autoHealMaxRestarts.error}
		class="p-0"
		lock={lock('autoHealMaxRestarts')}
		mixed={mixed('autoHealMaxRestarts')}
	>
		{#snippet children({ describedBy, errorId })}
			<Input
				id="auto-heal-max-restarts"
				type="number"
				min="1"
				bind:value={formInputs.autoHealMaxRestarts.value}
				aria-invalid={!!formInputs.autoHealMaxRestarts.error}
				placeholder={mixed('autoHealMaxRestarts') ? m.mixed() : undefined}
				aria-describedby={describedBy}
				aria-errormessage={errorId}
			/>
		{/snippet}
	</SettingsRow>
	<SettingsRow
		for="auto-heal-restart-window"
		label={m.auto_heal_restart_window_label()}
		description={m.auto_heal_restart_window_description()}
		error={formInputs.autoHealRestartWindow.error}
		class="p-0"
		lock={lock('autoHealRestartWindow')}
		mixed={mixed('autoHealRestartWindow')}
	>
		{#snippet children({ describedBy, errorId })}
			<Input
				id="auto-heal-restart-window"
				type="number"
				min="1"
				bind:value={formInputs.autoHealRestartWindow.value}
				aria-invalid={!!formInputs.autoHealRestartWindow.error}
				placeholder={mixed('autoHealRestartWindow') ? m.mixed() : undefined}
				aria-describedby={describedBy}
				aria-errormessage={errorId}
			/>
		{/snippet}
	</SettingsRow>
	<SettingsRow
		for="auto-heal-container-search"
		label={m.excluded_containers()}
		description={m.auto_heal_exclude_description()}
		layout="wide"
		class="p-0"
		lock={lock('autoHealExcludedContainers')}
		mixed={mixed('autoHealExcludedContainers')}
	>
		{#snippet children({ describedBy })}
			<div class="space-y-2">
				<Input
					id="auto-heal-container-search"
					type="search"
					placeholder={m.jobs_search_containers()}
					bind:value={autoHealSearchTerm}
					aria-describedby={describedBy}
				/>
				<div class="h-64 w-full overflow-hidden rounded-md border">
					<ScrollArea.Root class="h-full">
						<div class="space-y-2 p-2">
							{#if containersQuery.isLoading}
								<div class="flex items-center justify-center p-4">
									<Spinner class="size-4" />
								</div>
							{:else if containersQuery.isError}
								<div class="space-y-1 p-2 text-sm text-destructive">
									<p>{m.jobs_containers_load_error()}</p>
									<p class="text-xs opacity-80">{extractApiErrorMessage(containersQuery.error)}</p>
								</div>
							{:else}
								{@const names = containers
									.map(containerName)
									.filter((name) => name.toLowerCase().includes(autoHealSearchTerm.toLowerCase()))}
								{#each names as name (name)}
									<div class="flex items-center space-x-2">
										<Checkbox
											id="auto-heal-container-{name}"
											checked={autoHealExcluded.has(name)}
											onCheckedChange={() =>
												(formInputs.autoHealExcludedContainers.value = toggleExcluded(autoHealExcluded, name))}
										/>
										<Label for="auto-heal-container-{name}" weight="normal">{name}</Label>
									</div>
								{:else}
									<p class="py-4 text-center text-sm text-muted-foreground">{m.common_no_results_found()}</p>
								{/each}
							{/if}
						</div>
					</ScrollArea.Root>
				</div>
			</div>
		{/snippet}
	</SettingsRow>
{/snippet}

<div class="space-y-8">
	<SettingsSection title={m.maintenance()}>
		<SettingsRow
			for="scheduled-prune-enabled"
			label={m.scheduled_prune()}
			description={m.scheduled_prune_description()}
			layout="switch"
			lock={lock('scheduledPruneEnabled')}
			mixed={mixed('scheduledPruneEnabled')}
		>
			{#snippet children({ describedBy })}
				<Switch
					id="scheduled-prune-enabled"
					bind:checked={formInputs.scheduledPruneEnabled.value}
					data-mixed={!!mixed('scheduledPruneEnabled') || undefined}
					aria-describedby={describedBy}
				/>
			{/snippet}
			{#snippet labelExtra()}
				{@render scheduleControl('scheduledPruneInterval', m.scheduled_prune())}
			{/snippet}
		</SettingsRow>
	</SettingsSection>

	{#if vulnerabilityManagementEnabled}
		<SettingsSection title={m.security()}>
			<SettingsRow
				for="vulnerability-scan-enabled"
				label={m.activity_type_vulnerability_scan()}
				description={m.vulnerability_scan_description()}
				layout="switch"
				lock={lock('vulnerabilityScanEnabled')}
				mixed={mixed('vulnerabilityScanEnabled')}
			>
				{#snippet children({ describedBy })}
					<Switch
						id="vulnerability-scan-enabled"
						bind:checked={formInputs.vulnerabilityScanEnabled.value}
						data-mixed={!!mixed('vulnerabilityScanEnabled') || undefined}
						aria-describedby={describedBy}
					/>
				{/snippet}
				{#snippet labelExtra()}
					{@render scheduleControl('vulnerabilityScanInterval', m.activity_type_vulnerability_scan())}
				{/snippet}
			</SettingsRow>
		</SettingsSection>
	{/if}

	<SettingsSection title={m.jobs_monitoring_heading()}>
		<SettingsRow
			for="auto-heal-enabled"
			label={m.notifications_test_auto_heal_notification()}
			description={m.auto_heal_description()}
			layout="switch"
			lock={lock('autoHealEnabled')}
			mixed={mixed('autoHealEnabled')}
			expanded={formInputs.autoHealEnabled.value ? autoHealOptions : undefined}
		>
			{#snippet children({ describedBy })}
				<Switch
					id="auto-heal-enabled"
					bind:checked={formInputs.autoHealEnabled.value}
					data-mixed={!!mixed('autoHealEnabled') || undefined}
					aria-describedby={describedBy}
				/>
			{/snippet}
			{#snippet labelExtra()}
				{@render scheduleControl('autoHealInterval', m.notifications_test_auto_heal_notification())}
			{/snippet}
		</SettingsRow>
	</SettingsSection>
</div>

{#if scheduleTarget}
	{@const values = scheduleValues(scheduleTarget.key)}
	{#key scheduleTarget}
		<JobScheduleDialog
			name={scheduleTarget.name}
			settingsKey={scheduleTarget.key}
			schedule={scheduleTarget.value ?? (values.length > 1 ? '' : (values[0] ?? ''))}
			mixed={!scheduleTarget.value && values.length > 1}
			timezone={scheduleTimezone}
			save={saveSchedule}
			bind:open={scheduleOpen}
			onUpdate={() => (scheduleOpen = false)}
		/>
	{/key}
{/if}
