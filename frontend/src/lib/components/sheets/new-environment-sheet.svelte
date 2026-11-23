<script lang="ts">
	import { createMutation, createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { z } from 'zod/v4';

	import AgentCommandBlock from '#lib/components/agent-command-block.svelte';
	import { ArcaneButton } from '#lib/components/arcane-button/index.js';
	import FormInput from '#lib/components/form/form-input.svelte';
	import LabeledSwitch from '#lib/components/form/labeled-switch.svelte';
	import SelectWithLabel from '#lib/components/form/select-with-label.svelte';
	import { CopyButton } from '#lib/components/ui/copy-button/index.js';
	import * as ResponsiveDialog from '#lib/components/ui/responsive-dialog/index.js';
	import { Spinner } from '#lib/components/ui/spinner/index.js';
	import { AgentConnectionIcon } from '#lib/icons/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { environmentGroupListQuery } from '#lib/query/environment-groups.js';
	import { queryKeys } from '#lib/query/query-keys.js';
	import { environmentManagementService } from '#lib/services/env-mgmt-service.js';
	import { environmentStore } from '#lib/stores/environment.store.svelte.js';
	import { GLOBAL_SCOPE } from '#lib/types/role.js';
	import type { EnvironmentCreate } from '#lib/types/ui/environment.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';
	import { hasPermission, isGlobalAdmin } from '#lib/utils/auth.js';
	import { createForm, preventDefault } from '#lib/utils/settings.svelte.js';

	type NewEnvironmentSheetProps = {
		open: boolean;
		onEnvironmentCreated?: () => void;
		/** Environment group the new agent starts in. */
		groupId?: string;
	};

	let { open = $bindable(false), onEnvironmentCreated, groupId = '' }: NewEnvironmentSheetProps = $props();
	const queryClient = useQueryClient();

	let createdEnvironment = $state<{ id: string; useMTLS: boolean } | null>(null);

	const snippetsMutation = createMutation(() => ({
		mutationFn: (environmentId: string) => environmentManagementService.generateDeploymentSnippets(environmentId)
	}));
	const mtlsAssetsUnavailable = $derived(!!createdEnvironment?.useMTLS && !!snippetsMutation.data && !snippetsMutation.data.mtls);
	const generatedMTLS = $derived(createdEnvironment?.useMTLS ? snippetsMutation.data?.mtls : undefined);
	const createdSnippets = $derived(generatedMTLS ?? snippetsMutation.data);

	const createEnvironmentMutation = createMutation(() => ({
		mutationFn: (variables: { dto: EnvironmentCreate; useMTLS: boolean }) => environmentManagementService.create(variables.dto),
		onSuccess: async (created, variables) => {
			await environmentStore.initialize([...environmentStore.available, created]);
			await queryClient.invalidateQueries({ queryKey: queryKeys.environments.all });
			createdEnvironment = { id: created.id, useMTLS: variables.useMTLS };
			snippetsMutation.mutate(created.id);
			toast.success(m.environments_created_success());
		},
		onError: (error) => {
			toast.error(m.environments_create_failed(), { description: extractApiErrorMessage(error) });
		}
	}));
	const isSubmittingNewAgent = $derived(createEnvironmentMutation.isPending);

	let agentMTLSEnabled = $state(false);

	const agentFormSchema = z.object({
		name: z.string().min(1, m.environments_name_required()).max(25, m.environments_name_too_long()),
		groupId: z.string()
	});

	const agentForm = createForm<typeof agentFormSchema>(agentFormSchema, {
		name: '',
		groupId: untrack(() => groupId)
	});
	let agentInputs = $derived(agentForm.inputs);

	const environmentGroupsQuery = createQuery(() => ({
		...environmentGroupListQuery(),
		enabled: hasPermission('environments:list', GLOBAL_SCOPE)
	}));
	const environmentGroupOptions = $derived([
		{ value: '', label: m.none() },
		...(environmentGroupsQuery.data ?? []).map((group) => ({ value: group.id, label: group.name }))
	]);

	function handleAgentSubmit() {
		const data = agentForm.validate();
		if (!data) return;

		createEnvironmentMutation.mutate({
			dto: { name: data.name, groupId: data.groupId || undefined },
			useMTLS: agentMTLSEnabled
		});
	}

	function handleDone() {
		onEnvironmentCreated?.();
		open = false;
	}
</script>

<ResponsiveDialog.Root
	bind:open
	variant="sheet"
	title={createdEnvironment ? m.environments_created_title() : m.environments_create_new_agent()}
	description={createdEnvironment ? m.environments_created_description() : m.environments_create_new_agent_description()}
	contentClass="sm:max-w-2xl"
>
	<div class="space-y-6 px-6 py-6">
		{#if createdEnvironment}
			<div class="space-y-4">
				<div class="flex items-center gap-2 rounded-lg bg-primary/10 p-3 text-sm text-primary">
					<AgentConnectionIcon class="size-5" />
					<span>{m.env_agent_desc()}</span>
				</div>

				{#if snippetsMutation.data}
					<div class="space-y-2">
						<div class="text-sm font-medium">{m.enrollment_token()}</div>
						<div class="flex items-center gap-2">
							<code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all">
								{snippetsMutation.data.enrollmentToken}
							</code>
							<CopyButton text={snippetsMutation.data.enrollmentToken} size="icon" class="size-7" />
						</div>
						<p class="text-xs text-muted-foreground">{m.enrollment_token_single_use_description()}</p>
					</div>
				{/if}

				{#if generatedMTLS}
					<div class="space-y-3 rounded-lg border border-info/25 bg-info/8 p-4">
						<div class="space-y-1">
							<p class="text-sm font-medium">{m.environments_new_agent_mtls_enabled()}</p>
							<p class="text-xs text-muted-foreground">
								{m.environments_new_agent_mtls_auto_enroll()}
							</p>
						</div>
					</div>
				{/if}

				{#if mtlsAssetsUnavailable}
					<p class="text-sm text-warning">{m.environments_new_agent_mtls_assets_unavailable()}</p>
				{/if}

				{#if snippetsMutation.isPending}
					<div class="flex items-center justify-center py-8">
						<Spinner class="size-6" />
					</div>
				{:else if snippetsMutation.isError}
					<p class="text-sm text-destructive">{extractApiErrorMessage(snippetsMutation.error)}</p>
				{:else if createdSnippets?.dockerRun && createdSnippets.dockerCompose}
					<AgentCommandBlock
						dockerRunLabel={m.docker_run_command()}
						dockerRun={createdSnippets.dockerRun}
						dockerComposeLabel={m.common_docker_compose()}
						dockerCompose={createdSnippets.dockerCompose}
					/>
				{/if}

				<ArcaneButton action="base" class="w-full" onclick={handleDone} customLabel={m.common_done()} />
			</div>
		{:else}
			<p class="text-sm text-muted-foreground">
				{m.environments_agent_desc()}
			</p>
			<form onsubmit={preventDefault(handleAgentSubmit)} class="space-y-4">
				<FormInput label={m.common_name()} placeholder={m.env_remote_docker_host()} bind:input={agentInputs.name} />
				<!-- Placing an agent in an environment extends that environment's access grants, so it is admin-only. -->
				{#if isGlobalAdmin()}
					<SelectWithLabel
						id="new-agent-environment"
						label={m.environment()}
						description={m.agent_environment_description()}
						placeholder={m.none()}
						options={environmentGroupOptions}
						error={agentInputs.groupId.error}
						bind:value={agentInputs.groupId.value}
					/>
				{/if}

				<LabeledSwitch
					id="new-agent-mtls"
					label={m.environments_agent_mtls_enable()}
					description={m.environments_agent_mtls_desc()}
					checked={agentMTLSEnabled}
					onCheckedChange={(checked) => (agentMTLSEnabled = checked)}
					disabled={isSubmittingNewAgent}
				/>

				<ArcaneButton
					action="confirm"
					type="submit"
					class="w-full"
					disabled={isSubmittingNewAgent}
					loading={isSubmittingNewAgent}
					customLabel={m.environments_generate_config()}
				/>
			</form>
		{/if}
	</div>
</ResponsiveDialog.Root>
