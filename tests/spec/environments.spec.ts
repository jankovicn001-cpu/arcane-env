import { waitForDialogReady } from '../utils/playwright.util';
import { removeCreatedEnvironments, readApiData, readList } from '../utils/fetch.util';
import { ROW_ACTIONS_TRIGGER_NAME } from '../utils/table-actions.util';
import type { ContainerSummary } from '../types/containers.type';
import {
	test,
	expect,
	type Locator,
	type Page,
	type Request,
	type Route
} from '../fixtures/test.fixture';
import { expect as vitestExpect } from 'vite-plus/test';

type DeploymentSnippet = {
	enrollmentToken: string;
	dockerRun: string;
	dockerCompose: string;
};

const productionGroup = {
	id: 'e2e-group-production',
	name: 'Production',
	description: 'Customer-facing agents'
};

const stagingGroup = { id: 'e2e-group-staging', name: 'Staging', description: '' };

function inGroup<T extends object>(environment: T, group: typeof productionGroup) {
	return { ...environment, groupId: group.id, group };
}

function groupDto(group: typeof productionGroup, environmentCount: number) {
	const timestamp = '2026-01-01T00:00:00Z';
	return { ...group, environmentCount, createdAt: timestamp, updatedAt: timestamp };
}

test.describe('Settings', () => {
	const LOCAL_ENV_ID = '0';
	const NAME_PLACEHOLDER = 'My Lab Server';

	async function openEnvironment(page: Page, environmentId: string) {
		await page.goto(`/agents/${environmentId}`);
		await page.waitForLoadState('load');
		await expect(environmentTitleButton(page)).toBeVisible();
		await expect(page.getByRole('button', { name: 'Save', exact: true }).first()).toBeVisible();
	}

	function environmentTitleButton(page: Page) {
		return page.locator('h1 button').first();
	}

	async function renameEnvironmentInHeader(page: Page, newName: string) {
		await environmentTitleButton(page).click();
		const nameInput = page.getByPlaceholder(NAME_PLACEHOLDER);
		await expect(nameInput).toBeVisible();
		await nameInput.fill(newName);
		await nameInput.press('Enter');
		await expect(environmentTitleButton(page)).toHaveText(newName);
	}

	async function createAgentEnvironmentViaUI(
		page: Page,
		environmentName: string,
		environmentIds: Set<string>
	) {
		await page.goto('/agents');
		await page.waitForLoadState('load');

		await page.getByRole('button', { name: 'Add Agent', exact: true }).click();
		await expect(page.getByText('Create New Agent')).toBeVisible();

		const dialog = page.getByRole('dialog');
		await waitForDialogReady(dialog);
		await dialog.getByPlaceholder('Remote Docker Host').fill(environmentName);
		const createResponsePromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/agents'
		);
		const deploymentResponsePromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'POST' &&
				new URL(response.url()).pathname.startsWith('/api/agents/') &&
				new URL(response.url()).pathname.endsWith('/deployment')
		);
		await page.getByRole('button', { name: 'Generate Agent Configuration', exact: true }).click();
		const createResponse = await createResponsePromise;
		expect(createResponse.ok(), 'Create agent environment').toBeTruthy();
		expect(createResponse.request().postDataJSON()).toEqual({ name: environmentName });
		const created: { data: { id: string; apiKey?: string } } = await createResponse.json();
		environmentIds.add(created.data.id);
		expect(created.data.id).toBeTruthy();
		expect(created.data.apiKey).toBeUndefined();

		const deploymentResponse = await deploymentResponsePromise;
		expect(new URL(deploymentResponse.url()).pathname).toBe(
			`/api/agents/${created.data.id}/deployment`
		);
		expect(deploymentResponse.ok(), 'Generate deployment snippets').toBeTruthy();
		const deployment = await readApiData<DeploymentSnippet>(
			deploymentResponse,
			'Generate deployment snippets'
		);
		expect(deployment.enrollmentToken.startsWith('arc_etkn_')).toBe(true);
		expect(deployment.dockerRun).toContain(`ARCANE_AGENT_TOKEN=${deployment.enrollmentToken}`);
		expect(deployment.dockerCompose).toContain(`ARCANE_AGENT_TOKEN=${deployment.enrollmentToken}`);

		await expect(
			page.getByRole('heading', { name: 'Agent Created Successfully', exact: true })
		).toBeVisible();
		await expect(dialog.getByText(deployment.enrollmentToken, { exact: true })).toBeVisible();
		await expect(
			dialog.locator('pre code').filter({ hasText: 'ARCANE_AGENT_TOKEN=' }).first()
		).toBeVisible();
		await page.getByRole('button', { name: 'Done', exact: true }).click();
		await expect(page.getByRole('button', { name: environmentName, exact: true })).toBeVisible();
		return { id: created.data.id, enrollmentToken: deployment.enrollmentToken };
	}

	async function openLocalSettings(page: Page, category: string) {
		await page.goto(`/settings/${category}?env=${LOCAL_ENV_ID}`);
		await page.waitForLoadState('load');
		await expect(page.getByText('Applying settings for', { exact: true })).toBeVisible();
	}

	async function saveAndWaitForPatch(page: Page, expectedPath: string) {
		const saveButton = page.getByRole('button', { name: 'Save', exact: true }).first();
		await expect(saveButton).toBeEnabled();

		const responsePromise = page.waitForResponse((response) => {
			const request = response.request();
			if (request.method() !== 'PATCH') return false;
			const url = new URL(response.url());
			return url.pathname === expectedPath;
		});

		await saveButton.click();
		const response = await responsePromise;
		expect(response.ok(), `Expected successful PATCH to ${expectedPath}`).toBeTruthy();
		await expect(saveButton).toBeDisabled({ timeout: 10000 });
		return response.request();
	}

	async function selectSettingOption(page: Page, trigger: Locator, optionText: string) {
		await expect(trigger).toBeVisible();
		await trigger.click();
		const option = page.getByRole('option').filter({ hasText: optionText }).first();
		await expect(option).toBeVisible();
		await option.click();
	}

	test.describe('Environment Settings UI', () => {
		const settingKeys = new Set([
			'baseServerUrl',
			'followProjectSymlinks',
			'defaultDeployPullPolicy',
			'trivyNetwork',
			'trivyResourceLimitsEnabled',
			'trivyMemoryLimitMb',
			'trivyCpuLimit'
		]);
		let originalSettings: Record<string, string> = {};
		test.beforeEach(async ({ page }) => {
			originalSettings = {};
			const response = await page.request.get(`/api/agents/${LOCAL_ENV_ID}/settings`);
			expect(response.ok(), 'Read original environment settings').toBe(true);
			const { settings }: { settings: Array<{ key: string; value: string }> } =
				await response.json();
			expect(Array.isArray(settings)).toBe(true);
			originalSettings = Object.fromEntries(
				settings
					.filter((setting) => settingKeys.has(setting.key))
					.map((setting) => [setting.key, setting.value])
			);
		});
		test.afterEach(async ({ page }) => {
			if (Object.keys(originalSettings).length === 0) return;
			try {
				const currentResponse = await page.request.get(`/api/agents/${LOCAL_ENV_ID}/settings`);
				expect(currentResponse.ok()).toBe(true);
				const { settings: current }: { settings: Array<{ key: string; value: string }> } =
					await currentResponse.json();
				const changed = Object.fromEntries(
					Object.entries(originalSettings).filter(
						([key, value]) => current.find((setting) => setting.key === key)?.value !== value
					)
				);
				if (Object.keys(changed).length === 0) return;
				const response = await page.request.patch(`/api/agents/${LOCAL_ENV_ID}/settings`, {
					data: changed
				});
				expect(
					response.ok(),
					`Restore environment settings: ${response.status()} ${await response.text()}`
				).toBe(true);
				const restoredResponse = await page.request.get(`/api/agents/${LOCAL_ENV_ID}/settings`);
				expect(restoredResponse.ok()).toBe(true);
				const { settings: restored }: { settings: Array<{ key: string; value: string }> } =
					await restoredResponse.json();
				expect(
					Object.fromEntries(
						restored
							.filter((setting) => settingKeys.has(setting.key))
							.map((setting) => [setting.key, setting.value])
					)
				).toEqual(originalSettings);
			} catch (error) {
				vitestExpect.soft(false, `Restore environment settings: ${String(error)}`).toBe(true);
			}
		});

		test('should keep primary tabs selectable and restore them from the URL', async ({ page }) => {
			await page.goto(`/agents/${LOCAL_ENV_ID}?tab=engine`);
			await page.waitForLoadState('load');

			await expect.poll(() => new URL(page.url()).searchParams.get('tab')).toBe('engine');
			await expect(page.getByRole('tab', { name: 'Engine', exact: true })).toHaveAttribute(
				'data-state',
				'active'
			);

			const featuresTab = page.getByRole('tab', { name: 'Features', exact: true });
			await featuresTab.click();
			await expect.poll(() => new URL(page.url()).searchParams.get('tab')).toBe('features');
			await page.reload();
			await expect(featuresTab).toHaveAttribute('data-state', 'active');

			await page.goto(`/agents/${LOCAL_ENV_ID}?source=e2e&tab=invalid#tab-state`);
			await expect.poll(() => new URL(page.url()).searchParams.get('tab')).toBe('features');
			const canonicalUrl = new URL(page.url());
			expect(canonicalUrl.searchParams.get('source')).toBe('e2e');
			expect(canonicalUrl.hash).toBe('#tab-state');
			await expect(page.getByRole('tab', { name: 'Features', exact: true })).toHaveAttribute(
				'data-state',
				'active'
			);
			await expect(page.getByRole('heading', { name: 'Features', exact: true })).toBeVisible();
			await expect(
				page.getByRole('switch', { name: 'Vulnerability management', exact: true })
			).toBeVisible();
		});

		test('should show job run history on the History tab', async ({ page }) => {
			await page.goto(`/agents/${LOCAL_ENV_ID}?tab=history&job=event-cleanup`);
			await expect(page.getByRole('tab', { name: 'History', exact: true })).toHaveAttribute(
				'data-state',
				'active'
			);
			await expect(page.getByRole('heading', { name: 'Run history', exact: true })).toBeVisible();
			await expect(page.locator('#job-history-job')).toContainText('Event Cleanup');
		});

		test(
			'should update and save environment details',
			{ timeout: 120_000 },
			async ({ page, registerCleanup }) => {
				const envName = `settings-ui-${Date.now().toString().slice(-5)}`;
				const updatedName = `${envName}-updated`;
				const activityStatuses = new Map<string, number>();
				page.on('response', (response) => {
					const match = new URL(response.url()).pathname.match(
						/^\/api\/agents\/([^/]+)\/activities$/
					);
					if (match) activityStatuses.set(match[1], response.status());
				});
				const environmentIds = new Set<string>();

				registerCleanup(() => removeCreatedEnvironments(page, environmentIds, envName));
				const { id: environmentId, enrollmentToken } = await createAgentEnvironmentViaUI(
					page,
					envName,
					environmentIds
				);
				const environmentPath = `/api/agents/${environmentId}`;
				await expect.poll(() => activityStatuses.get(environmentId)).toBe(200);
				expect((await page.request.get('/api/auth/me', { timeout: 5_000 })).status()).toBe(200);
				await expect(page).toHaveURL('/agents?tab=environments');
				await page.getByRole('button', { name: envName, exact: true }).click();
				await expect(page).toHaveURL(/\/agents\/[^/?]+\?tab=[a-z]+$/);
				await expect(page.getByText(/^agent:\/\/settings-ui-/).first()).toBeVisible();

				await renameEnvironmentInHeader(page, updatedName);
				const nameRequest = await saveAndWaitForPatch(page, environmentPath);
				expect(nameRequest.postDataJSON()).toEqual({ name: updatedName });

				await page.reload();
				await expect(environmentTitleButton(page)).toHaveText(updatedName);

				await page.getByRole('tab', { name: 'Connection & Agent', exact: true }).click();
				const deploymentPath = `${environmentPath}/deployment`;
				await page.getByRole('button', { name: 'Generate Enrollment Token', exact: true }).click();
				const tokenDialog = page.getByRole('dialog');
				await expect(
					tokenDialog.getByRole('heading', { name: 'Generate Enrollment Token', exact: true })
				).toBeVisible();
				const tokenResponsePromise = page.waitForResponse(
					(response) =>
						response.request().method() === 'POST' &&
						new URL(response.url()).pathname === deploymentPath
				);
				await tokenDialog
					.getByRole('button', { name: 'Generate Enrollment Token', exact: true })
					.click();
				const tokenResponse = await tokenResponsePromise;
				expect(tokenResponse.ok(), 'Generate enrollment token').toBeTruthy();
				const generated = await readApiData<DeploymentSnippet>(
					tokenResponse,
					'Generate enrollment token'
				);
				expect(generated.enrollmentToken.startsWith('arc_etkn_')).toBe(true);
				expect(generated.enrollmentToken === enrollmentToken).toBe(false);

				await expect(page.getByText(generated.enrollmentToken, { exact: true })).toBeVisible();
				const generatedSnippet = page
					.locator('pre code')
					.filter({ hasText: `ARCANE_AGENT_TOKEN=${generated.enrollmentToken}` });
				await expect(generatedSnippet.first()).toBeVisible();
				await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
				await expect(page.getByText(generated.enrollmentToken, { exact: true })).toBeHidden();

				await renameEnvironmentInHeader(page, `${updatedName}-again`);
				const secondNameRequest = await saveAndWaitForPatch(page, environmentPath);
				expect(secondNameRequest.postDataJSON()).toEqual({ name: `${updatedName}-again` });
			}
		);

		test('should update and save the base server URL in Docker settings', async ({ page }) => {
			await openLocalSettings(page, 'docker');

			const baseServerUrlInput = page.locator('#base-server-url');
			await expect(baseServerUrlInput).toBeVisible();

			const originalBaseServerUrl = await baseServerUrlInput.inputValue();
			const updatedBaseServerUrl = originalBaseServerUrl.endsWith('/e2e')
				? `${originalBaseServerUrl}-2`
				: `${originalBaseServerUrl}/e2e`;

			try {
				await baseServerUrlInput.fill(updatedBaseServerUrl);
				await expect(baseServerUrlInput).toHaveValue(updatedBaseServerUrl);
				await saveAndWaitForPatch(page, `/api/agents/${LOCAL_ENV_ID}/settings`);

				await page.reload();
				await expect(page.locator('#base-server-url')).toHaveValue(updatedBaseServerUrl, {
					timeout: 15000
				});
			} finally {
				if (!page.isClosed()) {
					const currentValue = await page.locator('#base-server-url').inputValue();
					if (currentValue !== originalBaseServerUrl) {
						await page.locator('#base-server-url').fill(originalBaseServerUrl);
						await saveAndWaitForPatch(page, `/api/agents/${LOCAL_ENV_ID}/settings`);
					}
				}
			}
		});

		test('should update and save the follow project symlinks setting from Storage & Limits', async ({
			page
		}) => {
			await openLocalSettings(page, 'storage');

			const followProjectSymlinksSwitch = page.locator('#follow-project-symlinks');
			await expect(followProjectSymlinksSwitch).toBeVisible();

			const originalChecked =
				(await followProjectSymlinksSwitch.getAttribute('aria-checked')) === 'true';
			const updatedChecked = !originalChecked;

			try {
				await followProjectSymlinksSwitch.click();
				await expect(followProjectSymlinksSwitch).toHaveAttribute(
					'aria-checked',
					String(updatedChecked)
				);

				const saveButton = page.getByRole('button', { name: 'Save', exact: true }).first();
				await expect(saveButton).toBeEnabled();

				const responsePromise = page.waitForResponse((response) => {
					const request = response.request();
					if (request.method() !== 'PATCH') return false;
					const url = new URL(response.url());
					return url.pathname === `/api/agents/${LOCAL_ENV_ID}/settings`;
				});

				await saveButton.click();
				const response = await responsePromise;
				expect(response.ok()).toBeTruthy();

				await page.reload();
				await expect(page.locator('#follow-project-symlinks')).toHaveAttribute(
					'aria-checked',
					String(updatedChecked)
				);
			} finally {
				if (!page.isClosed()) {
					const currentChecked =
						(await page.locator('#follow-project-symlinks').getAttribute('aria-checked')) ===
						'true';
					if (currentChecked !== originalChecked) {
						await page.locator('#follow-project-symlinks').click();
						await saveAndWaitForPatch(page, `/api/agents/${LOCAL_ENV_ID}/settings`);
					}
				}
			}
		});

		test('should reset unsaved environment detail changes', async ({ page, registerCleanup }) => {
			const envName = `settings-reset-${Date.now().toString().slice(-5)}`;
			const environmentIds = new Set<string>();
			registerCleanup(() => removeCreatedEnvironments(page, environmentIds, envName));
			const { id: environmentId } = await createAgentEnvironmentViaUI(
				page,
				envName,
				environmentIds
			);
			await openEnvironment(page, environmentId);
			const titleButton = environmentTitleButton(page);
			await renameEnvironmentInHeader(page, `${envName}-pending`);

			const saveButton = page.getByRole('button', { name: 'Save', exact: true }).first();
			const resetButton = page.getByRole('button', { name: 'Reset', exact: true }).first();
			await expect(saveButton).toBeEnabled();
			await expect(resetButton).toBeVisible();
			await resetButton.click();
			await expect(titleButton).toHaveText(envName);
			await expect(saveButton).toBeDisabled();

			await renameEnvironmentInHeader(page, `${envName}-discarded`);
			await expect(saveButton).toBeEnabled();
			await page.getByRole('link', { name: 'Back', exact: true }).first().click();
			await expect(page).toHaveURL('/agents?tab=environments');
			await expect(page.getByRole('heading', { name: 'Agents', exact: true })).toBeVisible();
			await page.getByRole('button', { name: envName, exact: true }).click();
			await expect(page).toHaveURL(new RegExp(`/agents/${environmentId}(?:\\?|$)`));
			await expect(environmentTitleButton(page)).toHaveText(envName);
		});

		test('should update and save the default deploy pull policy in Docker settings', async ({
			page
		}) => {
			await openLocalSettings(page, 'docker');

			const pullPolicyTrigger = page.locator('#defaultDeployPullPolicy');
			await expect(pullPolicyTrigger).toBeVisible();

			const originalValue = (await pullPolicyTrigger.textContent())?.trim() || 'Missing';
			const updatedValue = originalValue.includes('Always') ? 'Never' : 'Always';

			try {
				await selectSettingOption(page, pullPolicyTrigger, updatedValue);
				await expect(pullPolicyTrigger).toContainText(updatedValue);
				await saveAndWaitForPatch(page, `/api/agents/${LOCAL_ENV_ID}/settings`);

				await page.reload();
				await expect(page.locator('#defaultDeployPullPolicy')).toContainText(updatedValue, {
					timeout: 15000
				});
			} finally {
				if (!page.isClosed()) {
					const currentValue = (
						(await page.locator('#defaultDeployPullPolicy').textContent()) || ''
					).trim();
					if (!currentValue.includes(originalValue)) {
						await selectSettingOption(
							page,
							page.locator('#defaultDeployPullPolicy'),
							originalValue
						);
						await saveAndWaitForPatch(page, `/api/agents/${LOCAL_ENV_ID}/settings`);
					}
				}
			}
		});

		test(
			'should save decimal trivy CPU limits and reject negative values',
			{ timeout: 120_000 },
			async ({ page }) => {
				const settingsPath = `/api/agents/${LOCAL_ENV_ID}/settings`;
				const settingsResponse = await page.request.get(settingsPath);
				expect(settingsResponse.ok()).toBeTruthy();
				const { settings } = (await settingsResponse.json()) as {
					settings: Array<{ key: string; value: string }>;
				};
				const originalSettings = Object.fromEntries(
					settings
						.filter(({ key }) =>
							['trivyResourceLimitsEnabled', 'trivyCpuLimit', 'trivyMemoryLimitMb'].includes(key)
						)
						.map(({ key, value }) => [key, value])
				);
				expect(Object.keys(originalSettings)).toHaveLength(3);

				try {
					await openLocalSettings(page, 'security');
					await page.locator('#trivyResourceLimitsEnabledSwitch').setChecked(true);
					const cpuInput = page.getByRole('spinbutton', { name: 'CPU Limit (cores)', exact: true });
					await expect(cpuInput).toHaveAttribute('min', '0');
					await expect(cpuInput).toHaveAttribute('step', 'any');

					const values = ['0.5', '1.5', '2.5', '0.25', '1', '0'];
					if ((await cpuInput.inputValue()) === values[0]) values.reverse();

					for (const value of values) {
						await cpuInput.fill(value);
						expect(await cpuInput.evaluate((input: HTMLInputElement) => input.validity.valid)).toBe(
							true
						);
						const requestPromise = page.waitForRequest(
							(request) =>
								request.method() === 'PATCH' && new URL(request.url()).pathname === settingsPath
						);
						await saveAndWaitForPatch(page, settingsPath);
						const payload = (await requestPromise).postDataJSON() as Record<string, unknown>;
						expect(payload.trivyCpuLimit).toBe(value);
						expect(payload.trivyResourceLimitsEnabled ?? 'true').toBe('true');
						await page.reload();
						await expect(cpuInput).toHaveValue(value);
					}

					const writes: Request[] = [];
					const recordWrite = (request: Request) => {
						if (request.method() === 'PATCH' && new URL(request.url()).pathname === settingsPath) {
							writes.push(request);
						}
					};
					page.on('request', recordWrite);
					try {
						await cpuInput.fill('-0.5');
						await page.getByRole('button', { name: 'Save', exact: true }).first().click();
						await expect(
							page.getByText('Please check the form for errors.', { exact: true })
						).toBeVisible();
						expect(writes).toHaveLength(0);
						await page.reload();
						await expect(cpuInput).toHaveValue(values[values.length - 1]);
					} finally {
						page.off('request', recordWrite);
					}
				} finally {
					const restored = await page.request.patch(settingsPath, { data: originalSettings });
					expect(restored.ok()).toBeTruthy();
				}
			}
		);

		test('should update and save the trivy network mode including auto', async ({ page }) => {
			await openLocalSettings(page, 'security');

			const trivyNetworkTrigger = page.locator('#trivyNetwork');
			await expect(trivyNetworkTrigger).toBeVisible();

			const originalValue = ((await trivyNetworkTrigger.textContent()) || '').trim();
			const updatedValue = originalValue.includes('bridge') ? 'Auto' : 'bridge';

			try {
				await selectSettingOption(page, trivyNetworkTrigger, updatedValue);
				await expect(trivyNetworkTrigger).toContainText(updatedValue);

				const saveButton = page.getByRole('button', { name: 'Save', exact: true }).first();
				await expect(saveButton).toBeEnabled();

				const responsePromise = page.waitForResponse((response) => {
					const request = response.request();
					if (request.method() !== 'PATCH') return false;
					const url = new URL(response.url());
					return url.pathname === `/api/agents/${LOCAL_ENV_ID}/settings`;
				});

				await saveButton.click();
				const response = await responsePromise;
				expect(response.ok()).toBeTruthy();

				await page.reload();
				await expect(page.locator('#trivyNetwork')).toContainText(updatedValue);
			} finally {
				if (!page.isClosed()) {
					const currentValue = ((await page.locator('#trivyNetwork').textContent()) || '').trim();
					if (!currentValue.includes(originalValue)) {
						await selectSettingOption(page, page.locator('#trivyNetwork'), originalValue);
						await saveAndWaitForPatch(page, `/api/agents/${LOCAL_ENV_ID}/settings`);
					}
				}
			}
		});
	});

	test('the Environment select assigns the agent to an environment group', async ({ page }) => {
		const local = await readApiData<Record<string, unknown>>(
			await page.request.get(`/api/agents/${LOCAL_ENV_ID}`),
			'Get local environment'
		);
		let current: Record<string, unknown> = { ...local, groupId: undefined, group: undefined };
		await page.route(/\/api\/environment-groups(?:\?.*)?$/, (route) =>
			route.fulfill({
				json: { success: true, data: [groupDto(productionGroup, 0), groupDto(stagingGroup, 0)] }
			})
		);
		// Keeps the real agent untouched: the PATCH is answered and later reads return the update.
		await page.route(new RegExp(`/api/agents/${LOCAL_ENV_ID}$`), async (route) => {
			if (route.request().method() === 'PATCH') {
				current = inGroup(current, productionGroup);
			}
			await route.fulfill({ json: { success: true, data: current } });
		});

		await openEnvironment(page, LOCAL_ENV_ID);
		const environmentSelect = page.locator('#environment-group');
		await expect(environmentSelect).toContainText('None');
		await selectSettingOption(page, environmentSelect, productionGroup.name);
		await expect(environmentSelect).toContainText(productionGroup.name);

		const request = await saveAndWaitForPatch(page, `/api/agents/${LOCAL_ENV_ID}`);
		expect(request.postDataJSON()).toEqual({ groupId: productionGroup.id });
		await expect(environmentSelect).toContainText(productionGroup.name);
	});
});

const localEnvironment = {
	id: '0',
	name: 'Local Test',
	apiUrl: 'unix:///var/run/docker.sock',
	status: 'online',
	enabled: true
};

const remoteEnvironment = {
	id: 'remote-switch-test',
	name: 'Remote Test',
	apiUrl: 'agent://remote-switch-test',
	status: 'online',
	enabled: true
};

function paginated<T>(data: T[]) {
	return {
		success: true,
		data,
		counts: {
			runningContainers: data.length,
			stoppedContainers: 0,
			totalContainers: data.length
		},
		pagination: {
			totalPages: 1,
			totalItems: data.length,
			currentPage: 1,
			itemsPerPage: 20,
			grandTotalItems: data.length
		}
	};
}

function containerSummary(id: string, name: string) {
	return {
		id,
		names: [name],
		image: 'nginx:latest',
		imageId: `sha256:${id}`,
		command: 'nginx',
		created: 1_700_000_000,
		labels: {},
		state: 'running',
		status: 'Up 5 minutes',
		ports: [],
		hostConfig: { networkMode: 'default' },
		networkSettings: { networks: {} },
		mounts: []
	};
}

async function mockEnvironmentCatalog(
	page: Page,
	environments: Array<{ id: string }> = [localEnvironment, remoteEnvironment]
) {
	await page.addInitScript(() => {
		localStorage.removeItem('arcane-container-table');
	});
	await page.context().route(/\/api\/agents(?:\?.*)?$/, async (route) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(paginated(environments))
		});
	});
	await page.context().route(/\/api\/auth\/bootstrap(?:\?.*)?$/, async (route) => {
		const response = await route.fetch();
		const body = await response.json();
		await route.fulfill({
			response,
			json: {
				...body,
				data: {
					...body.data,
					environments: paginated(environments)
				}
			}
		});
	});
	await page.context().route(/\/api\/stream(?:\?.*)?$/, async (route) => {
		const channels = new URL(route.request().url()).searchParams.get('channels')?.split(',') ?? [];
		const timestamp = new Date().toISOString();
		await route.fulfill({
			status: 200,
			contentType: 'text/event-stream',
			body: channels.includes('environments')
				? `data: ${JSON.stringify({
						channel: 'environments',
						environment: {
							type: 'snapshot',
							environments,
							timestamp
						},
						timestamp
					})}\n\n`
				: ''
		});
	});
	await page
		.context()
		.route(
			new RegExp(`/api/agents/(?:${environments.map((env) => env.id).join('|')})/settings$`),
			async (route) => {
				if (route.request().method() !== 'GET') {
					await route.continue();
					return;
				}
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({ settings: [], runtime: {}, readOnly: false })
				});
			}
		);
}

test.describe('Environment filter', () => {
	function containerDetails(id: string, name: string) {
		return {
			id,
			name,
			image: 'nginx:latest',
			imageId: `sha256:${id}`,
			created: '2026-01-01T00:00:00Z',
			state: {
				status: 'running',
				running: true,
				startedAt: '2026-01-01T00:00:00Z',
				finishedAt: ''
			},
			config: {},
			hostConfig: { networkMode: 'default' },
			networkSettings: { networks: {} },
			ports: [],
			mounts: [],
			labels: {},
			redeployDisabled: false
		};
	}

	async function filterToRemoteEnvironment(page: Page) {
		await page.getByTestId('facet-agent-trigger').click();
		await page.getByTestId(`facet-agent-option-${remoteEnvironment.id}`).click();
	}

	test.describe('Environment filter isolation', () => {
		test('clears local rows and stats targets while the remote response is delayed', async ({
			page
		}) => {
			let releaseRemote!: () => void;
			const remoteGate = new Promise<void>((resolve) => {
				releaseRemote = resolve;
			});
			let markRemoteStarted!: () => void;
			const remoteStarted = new Promise<void>((resolve) => {
				markRemoteStarted = resolve;
			});
			let remoteMarked = false;
			const websocketPaths: string[] = [];

			await mockEnvironmentCatalog(page);
			await page
				.context()
				.route(/\/api\/agents\/[^/]+\/containers(?:\?.*)?$/, async (route: Route) => {
					const pathname = new URL(route.request().url()).pathname;
					if (pathname === '/api/agents/remote-switch-test/containers') {
						if (!remoteMarked) {
							remoteMarked = true;
							markRemoteStarted();
						}
						await remoteGate;
						await route.fulfill({
							status: 200,
							contentType: 'application/json',
							body: JSON.stringify(
								paginated([containerSummary('remote-b-id', 'remote-b-container')])
							)
						});
						return;
					}
					await route.fulfill({
						status: 200,
						contentType: 'application/json',
						body: JSON.stringify(paginated([containerSummary('local-a-id', 'local-a-container')]))
					});
				});
			page.on('websocket', (socket) => websocketPaths.push(new URL(socket.url()).pathname));

			await page.goto('/containers');
			await expect(
				page.getByRole('link', { name: 'local-a-container', exact: true })
			).toBeVisible();

			try {
				await filterToRemoteEnvironment(page);
				await remoteStarted;
				await expect(
					page.getByRole('link', { name: 'local-a-container', exact: true })
				).toHaveCount(0);
				await page.waitForTimeout(200);
				expect(websocketPaths).not.toContain(
					'/api/agents/remote-switch-test/ws/containers/local-a-id/stats'
				);
			} finally {
				releaseRemote();
			}

			await expect(
				page.getByRole('link', { name: 'remote-b-container', exact: true })
			).toBeVisible();
			await expect(page.getByRole('link', { name: 'local-a-container', exact: true })).toHaveCount(
				0
			);
		});

		test('does not restore local rows when the remote request fails', async ({
			page,
			pageErrorGuard
		}) => {
			pageErrorGuard.allow('remote unavailable');
			await mockEnvironmentCatalog(page);
			await page
				.context()
				.route(/\/api\/agents\/[^/]+\/containers(?:\?.*)?$/, async (route: Route) => {
					const pathname = new URL(route.request().url()).pathname;
					if (pathname === '/api/agents/remote-switch-test/containers') {
						await route.fulfill({
							status: 503,
							contentType: 'application/json',
							body: JSON.stringify({ success: false, message: 'remote unavailable' })
						});
						return;
					}
					await route.fulfill({
						status: 200,
						contentType: 'application/json',
						body: JSON.stringify(paginated([containerSummary('local-a-id', 'local-a-container')]))
					});
				});

			await page.goto('/containers');
			await expect(
				page.getByRole('link', { name: 'local-a-container', exact: true })
			).toBeVisible();
			const failedRemoteRequest = page.waitForResponse((response) => {
				return (
					new URL(response.url()).pathname === '/api/agents/remote-switch-test/containers' &&
					response.status() === 503
				);
			});
			await filterToRemoteEnvironment(page);
			await failedRemoteRequest;
			await expect(page.getByRole('link', { name: 'local-a-container', exact: true })).toHaveCount(
				0
			);
		});

		test('remounts same-route details and navigates to the unwrapped redeployed container id', async ({
			page
		}) => {
			const detailsById = {
				'route-a': containerDetails('route-a', 'Route A Container'),
				'route-b': containerDetails('route-b', 'Route B Container'),
				'route-redeployed': containerDetails('route-redeployed', 'Redeployed Container')
			};

			await page.context().route(/\/api\/agents\/0\/settings$/, async (route) => {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({ settings: [], runtime: {}, readOnly: false })
				});
			});
			await page.context().route(/\/api\/agents\/0\/containers\/([^/?]+)$/, async (route) => {
				const id = decodeURIComponent(
					new URL(route.request().url()).pathname.split('/').pop() ?? ''
				);
				const details = detailsById[id as keyof typeof detailsById];
				await route.fulfill({
					status: details ? 200 : 404,
					contentType: 'application/json',
					body: JSON.stringify(
						details ? { success: true, data: details } : { success: false, message: 'not found' }
					)
				});
			});
			await page
				.context()
				.route(/\/api\/agents\/0\/containers\/route-b\/redeploy$/, async (route) => {
					await route.fulfill({
						status: 200,
						contentType: 'application/json',
						body: JSON.stringify({ success: true, data: detailsById['route-redeployed'] })
					});
				});

			await page.goto('/containers/route-a?env=0');
			await expect(page.getByRole('heading', { name: 'Route A Container' })).toBeVisible();
			await page.getByRole('tab', { name: 'Logs' }).click();
			await expect(page.getByRole('tab', { name: 'Logs' })).toHaveAttribute(
				'aria-selected',
				'true'
			);

			await page.evaluate(() => {
				const link = document.createElement('a');
				link.id = 'same-route-navigation';
				link.href = '/containers/route-b?env=0';
				link.textContent = 'navigate';
				document.body.append(link);
			});
			await page.locator('#same-route-navigation').click();
			await expect(page).toHaveURL(
				(url) =>
					url.pathname === '/containers/route-b' &&
					url.searchParams.get('env') === '0' &&
					url.searchParams.get('tab') === 'overview'
			);
			await expect(page.getByRole('heading', { name: 'Route B Container' })).toBeVisible();
			await expect(page.getByRole('tab', { name: 'Overview' })).toHaveAttribute(
				'aria-selected',
				'true'
			);

			await page.getByRole('button', { name: 'More actions', exact: true }).click();
			await page.getByRole('menuitem', { name: 'Redeploy', exact: true }).click();
			const dialog = page.getByRole('dialog');
			await dialog.getByRole('button', { name: 'Redeploy', exact: true }).click();
			await expect(page).toHaveURL(
				(url) =>
					url.pathname === '/containers/route-redeployed' &&
					url.searchParams.get('env') === '0' &&
					url.searchParams.get('tab') === 'overview'
			);
			await expect(page.getByRole('heading', { name: 'Redeployed Container' })).toBeVisible();
		});
	});
});

test.describe('Agents', () => {
	const ROUTES = {
		environments: '/agents'
	};

	async function openNewEnvironmentSheet(page: Page) {
		await page.goto(ROUTES.environments);
		await page.waitForLoadState('load');

		const addButton = page.getByRole('button', { name: 'Add Agent', exact: true });
		await expect(addButton).toBeVisible();
		await addButton.click();

		await expect(page.getByText('Create New Agent')).toBeVisible();
		await expect(page.getByText('Agent connects outbound to the manager.')).toBeVisible();
	}

	test.describe('Agent Environment', () => {
		test('should require a name before creating an agent environment', async ({ page }) => {
			await openNewEnvironmentSheet(page);
			let createRequests = 0;

			await page.route('**/api/agents', async (route) => {
				if (route.request().method() === 'POST') {
					createRequests += 1;
				}
				await route.continue();
			});

			await page.getByRole('button', { name: 'Generate Agent Configuration', exact: true }).click();
			await expect.poll(() => createRequests).toBe(0);
		});

		test('should create an agent environment and show deployment snippets', async ({
			page,
			registerCleanup
		}) => {
			const environmentName = `agent-${Date.now().toString().slice(-6)}`;
			let createdEnvironmentId = null as string | null;
			const environmentIds = new Set<string>();
			registerCleanup(() => removeCreatedEnvironments(page, environmentIds, environmentName));
			const agentSettingsRequests: string[] = [];
			page.on('request', (request) => {
				if (!createdEnvironmentId) return;
				const pathname = new URL(request.url()).pathname;
				const [, , , scope = '', ...rest] = pathname.split('/');
				const suffix = rest.join('/');
				if (
					pathname.startsWith('/api/agents/') &&
					decodeURIComponent(scope).split(',').includes(createdEnvironmentId) &&
					(suffix === 'settings' || suffix === 'settings/public')
				) {
					agentSettingsRequests.push(pathname);
				}
			});

			await page.route('**/api/agents', async (route) => {
				if (route.request().method() === 'POST') {
					const response = await route.fetch();
					const environment = await readApiData<{ id: string }>(
						response,
						'Create agent environment'
					);
					createdEnvironmentId = environment.id;
					environmentIds.add(environment.id);
					await route.fulfill({ response });
					return;
				}

				await route.continue();
			});

			await openNewEnvironmentSheet(page);

			await page.getByPlaceholder('Remote Docker Host').fill(environmentName);
			const submitButton = page.getByRole('button', {
				name: 'Generate Agent Configuration',
				exact: true
			});
			await submitButton.click();

			const sheetTitle = page.locator('[data-slot="sheet-title"]');
			await expect(sheetTitle).toContainText('Agent Created');
			await expect(
				page.getByText('Arcane Agent - connects outbound to manager', { exact: true })
			).toBeVisible();
			await expect(page.getByText('Enrollment Token', { exact: true })).toBeVisible();
			await expect(page.getByText('Docker Run Command', { exact: true })).toBeVisible();
			await expect(page.getByText('Docker Compose', { exact: true })).toBeVisible();

			const snippetBlocks = page.locator('pre code').filter({ hasText: 'ARCANE_AGENT=true' });
			await expect(snippetBlocks.first()).toBeVisible();

			const dockerRunSnippet = snippetBlocks.first();
			await expect(dockerRunSnippet).toContainText('arcane-agent');
			await expect(dockerRunSnippet).not.toContainText('-p 3553:3553');

			await page.getByRole('button', { name: 'Done', exact: true }).click();

			await expect(page.getByRole('button', { name: environmentName, exact: true })).toBeVisible();
			const environmentRow = page.locator('tr').filter({
				has: page.getByRole('button', { name: environmentName, exact: true })
			});
			await expect(environmentRow.getByText('agent://agent-').first()).toBeVisible();
			await expect(environmentRow.getByText('Agent', { exact: true })).toBeVisible();

			await page.getByRole('button', { name: environmentName, exact: true }).click();
			if (createdEnvironmentId) {
				await expect(page).toHaveURL(new RegExp(`/agents/${createdEnvironmentId}`));
			}

			await expect(page.getByText(/^agent:\/\/agent-/).first()).toBeVisible();
			// A new agent environment waits for its agent, and the connection view opens by default.
			await expect(page.getByText('Connecting', { exact: true })).toBeVisible();
			await expect(page.getByRole('heading', { name: 'Security', exact: true })).toBeVisible();
			await expect(
				page.getByRole('tab', { name: 'Connection & Agent', exact: true })
			).toHaveAttribute('data-state', 'active');
			expect(agentSettingsRequests).toEqual([]);
		});
	});

	test.describe('Environments tab', () => {
		test('lists the environment groups and creates one', async ({ page }) => {
			const groups = [groupDto(productionGroup, 2)];
			const created: Array<Record<string, unknown>> = [];
			await page.route(/\/api\/environment-groups(?:\?.*)?$/, async (route) => {
				if (route.request().method() !== 'POST') {
					await route.fulfill({ json: { success: true, data: groups } });
					return;
				}
				const body = route.request().postDataJSON() as { name: string; description: string };
				created.push(body);
				const group = groupDto({ id: 'e2e-group-created', ...body }, 0);
				groups.push(group);
				await route.fulfill({ status: 201, json: { success: true, data: group } });
			});

			await page.goto(`${ROUTES.environments}?tab=environment-groups`);
			const tabs = page.getByRole('tab');
			await expect(tabs.nth(0)).toHaveText('Agents');
			await expect(tabs.nth(1)).toHaveText('Environments');
			await expect(tabs.nth(2)).toHaveText('Enrollment Tokens');
			await expect(page.getByRole('tab', { name: 'Environments', exact: true })).toHaveAttribute(
				'data-state',
				'active'
			);

			const productionRow = page.getByRole('row').filter({ hasText: productionGroup.name });
			await expect(productionRow).toContainText(productionGroup.description);
			await expect(productionRow.getByRole('cell').nth(2)).toHaveText('2');

			await page.getByRole('button', { name: 'Create Environment', exact: true }).click();
			const dialog = page.getByRole('dialog');
			await waitForDialogReady(dialog);
			await dialog.getByLabel('Name', { exact: true }).fill('QA');
			await dialog.getByLabel('Description', { exact: true }).fill('Pre-release checks');
			await dialog.getByRole('button', { name: 'Create', exact: true }).click();

			await expect.poll(() => created).toEqual([{ name: 'QA', description: 'Pre-release checks' }]);
			await expect(dialog).toBeHidden();
			await expect(page.getByRole('row').filter({ hasText: 'Pre-release checks' })).toBeVisible();
		});
	});
});

test.describe('Environment containers', () => {
	const offlineEnvironment = inGroup(
		{
			id: 'offline-switch-test',
			name: 'Offline Test',
			apiUrl: 'agent://offline-switch-test',
			status: 'offline',
			enabled: true,
			lastSeen: new Date(Date.now() - 5 * 60_000).toISOString()
		},
		productionGroup
	);

	// Staging holds only a disabled agent, so it has nothing to browse.
	const disabledEnvironment = inGroup(
		{
			id: 'disabled-switch-test',
			name: 'Disabled Test',
			apiUrl: 'agent://disabled-switch-test',
			status: 'online',
			enabled: false
		},
		stagingGroup
	);

	const remoteEnvironments = [
		inGroup(remoteEnvironment, productionGroup),
		offlineEnvironment,
		disabledEnvironment
	];
	const remoteHub = `/environments/${productionGroup.id}/${remoteEnvironment.id}`;
	const offlineHub = `/environments/${productionGroup.id}/${offlineEnvironment.id}`;
	const localHub = '/environments/unassigned/0';
	const remoteContainer = containerSummary('remote-b-id', 'remote-b-container');
	const localCounts = { totalContainers: 37, runningContainers: 23, stoppedContainers: 14 };

	// Lists the real local environment beside mocked remote ones and serves the remote containers.
	async function mockEnvironments(page: Page) {
		const real = await readApiData<{ id: string; name: string }>(
			await page.request.get('/api/agents/0'),
			'Get local environment'
		);
		const local = { ...real, groupId: undefined, group: undefined };
		await mockEnvironmentCatalog(page, [local, ...remoteEnvironments]);
		for (const environment of remoteEnvironments) {
			await page.route(new RegExp(`/api/agents/${environment.id}(?:\\?.*)?$`), (route) =>
				route.fulfill({ json: { success: true, data: environment } })
			);
			await page.route(new RegExp(`/api/agents/${environment.id}/containers(?:\\?.*)?$`), (route) =>
				route.fulfill({
					json: paginated(environment.id === remoteEnvironment.id ? [remoteContainer] : [])
				})
			);
		}
		return local;
	}

	// Serves the real local containers with fixed counts and keeps the rows the API reported.
	async function captureLocalContainers(page: Page) {
		const listed: { containers?: ContainerSummary[] } = {};
		await page.route(
			(url) => url.pathname === '/api/agents/0/containers',
			async (route) => {
				const response = await route.fetch();
				listed.containers = (
					await readList<ContainerSummary>(response, 'List local containers')
				).data;
				await route.fulfill({
					response,
					json: { ...(await response.json()), counts: localCounts }
				});
			}
		);
		return listed;
	}

	function recordContainerListPaths(page: Page) {
		const paths: string[] = [];
		page.on('request', (request) => {
			const { pathname } = new URL(request.url());
			if (/^\/api\/agents\/[^/]+\/containers$/.test(pathname)) paths.push(pathname);
		});
		return paths;
	}

	function resourceRows(page: Page) {
		return page
			.getByRole('row')
			.filter({ has: page.getByRole('button', { name: ROW_ACTIONS_TRIGGER_NAME }) });
	}

	function containerName(container: ContainerSummary) {
		return container.names?.[0] || container.id.slice(0, 12);
	}

	async function expectNoEnvironmentColumn(page: Page) {
		await expect(
			page.getByRole('columnheader', { name: /^(Sort by )?(Agent|Environment)$/ })
		).toHaveCount(0);
	}

	function pageHeading(page: Page, name: string) {
		return page.locator('main').getByRole('heading', { level: 1, name, exact: true });
	}

	const remoteNetwork = {
		id: 'remote-network-id',
		name: 'remote-network',
		driver: 'bridge',
		scope: 'local',
		created: '2024-01-01T00:00:00Z',
		options: {},
		labels: {},
		ipam: { driver: 'default', config: [], options: {} },
		inUse: false,
		isDefault: false
	};

	async function mockRemoteNetworks(page: Page) {
		await page.route(
			new RegExp(`/api/agents/${remoteEnvironment.id}/networks(?:\\?.*)?$`),
			(route) =>
				route.fulfill({
					json: { ...paginated([remoteNetwork]), counts: { inuse: 0, unused: 1, total: 1 } }
				})
		);
	}

	// Serves an environment's containers as a 503 until the returned switch makes it reachable.
	async function mockUnreachableContainers(page: Page, environmentId: string, message: string) {
		const state = { reachable: false };
		await page.route(
			new RegExp(`/api/agents/${environmentId}/containers(?:\\?.*)?$`),
			async (route) => {
				if (state.reachable) {
					await route.fulfill({
						json: paginated(environmentId === remoteEnvironment.id ? [remoteContainer] : [])
					});
				} else {
					await route.fulfill({ status: 503, json: { success: false, message } });
				}
			}
		);
		return state;
	}

	// Signs in as the real user with only the given permissions, beside the mocked environments.
	async function mockUserPermissions(
		page: Page,
		environments: Array<{ id: string }>,
		permissionsByEnv: Record<string, string[]>
	) {
		const limited = (user: Record<string, unknown>) => ({
			...user,
			isGlobalAdmin: false,
			roleAssignments: [],
			permissionsByEnv
		});
		await page.context().route(/\/api\/auth\/bootstrap(?:\?.*)?$/, async (route) => {
			const response = await route.fetch();
			const body = await response.json();
			await route.fulfill({
				response,
				json: {
					...body,
					data: {
						...body.data,
						user: limited(body.data.user),
						environments: paginated(environments)
					}
				}
			});
		});
		await page.context().route(/\/api\/auth\/me$/, async (route) => {
			const response = await route.fetch();
			const body = await response.json();
			await route.fulfill({ response, json: { ...body, data: limited(body.data) } });
		});
	}

	function environmentTab(page: Page, name: string) {
		return page.getByRole('tab', { name, exact: true });
	}

	function hasTabParam(tab: string) {
		return (url: URL) => url.searchParams.get('tab') === tab;
	}

	test('the Resources sidebar group starts with Environments and lists the environment groups', async ({
		page
	}) => {
		const local = await mockEnvironments(page);
		await page.goto('/containers');

		const resources = page.locator('[data-slot="sidebar-group"]').filter({
			has: page.locator('[data-slot="sidebar-group-label"]', { hasText: /^Resources$/ })
		});
		const topLevelLinks = resources.locator('[data-slot="sidebar-menu-button"]');
		await expect(topLevelLinks.nth(0)).toHaveText('Environments');
		await expect(topLevelLinks.nth(1)).toHaveText('Containers');
		await expect(
			page.locator('[data-slot="sidebar"]').getByRole('link', { name: 'Agents', exact: true })
		).toHaveAttribute('href', '/agents');

		const environmentsLink = resources.getByRole('link', { name: 'Environments', exact: true });
		await expect(environmentsLink).toHaveAttribute('href', '/environments');
		await environmentsLink.click();
		await expect(page).toHaveURL('/environments');

		const environmentsItem = resources
			.locator('[data-slot="sidebar-menu-item"]')
			.filter({ has: environmentsLink });
		const children = environmentsItem.locator('[data-slot="sidebar-menu-sub"]').getByRole('link');
		await expect(children).toHaveCount(2);
		await expect(children.nth(0)).toHaveText(productionGroup.name);
		await expect(children.nth(0)).toHaveAttribute('href', `/environments/${productionGroup.id}`);
		await expect(children.nth(1)).toHaveText('Unassigned');
		await expect(children.nth(1)).toHaveAttribute('href', '/environments/unassigned');
		for (const name of [stagingGroup.name, local.name, remoteEnvironment.name]) {
			await expect(environmentsItem.getByRole('link', { name, exact: true })).toHaveCount(0);
		}

		await children.nth(0).click();
		await expect(page).toHaveURL(`/environments/${productionGroup.id}`);
		await expect(pageHeading(page, productionGroup.name)).toBeVisible();
	});

	test('the index shows a card per environment group with browsable agents and Unassigned last', async ({
		page
	}) => {
		const local = await mockEnvironments(page);
		await page.goto('/environments');

		const main = page.locator('main');
		await expect(pageHeading(page, 'Environments')).toBeVisible();
		const cards = main.locator('a[href^="/environments/"]');
		await expect(cards).toHaveCount(2);

		const production = cards.nth(0);
		await expect(production).toHaveAttribute('href', `/environments/${productionGroup.id}`);
		for (const text of [
			productionGroup.name,
			productionGroup.description,
			'2 agent(s)',
			'1 online',
			'1 offline'
		]) {
			await expect(production).toContainText(text);
		}
		const unassigned = cards.nth(1);
		await expect(unassigned).toHaveAttribute('href', '/environments/unassigned');
		for (const text of ['Unassigned', '1 agent(s)', '1 online', '0 offline']) {
			await expect(unassigned).toContainText(text);
		}
		await expect(main.getByRole('link').filter({ hasText: stagingGroup.name })).toHaveCount(0);

		const search = main.getByRole('textbox', { name: 'Search', exact: true });
		await search.fill(offlineEnvironment.name);
		await expect(cards).toHaveCount(1);
		await expect(cards.first()).toHaveAttribute('href', `/environments/${productionGroup.id}`);
		await search.fill(local.name);
		await expect(cards).toHaveCount(1);
		await expect(cards.first()).toHaveAttribute('href', '/environments/unassigned');
		await search.fill('production');
		await expect(cards).toHaveCount(1);
		await expect(cards.first()).toHaveAttribute('href', `/environments/${productionGroup.id}`);

		await search.fill('');
		await production.click();
		await expect(page).toHaveURL(`/environments/${productionGroup.id}`);
		await expect(pageHeading(page, productionGroup.name)).toBeVisible();
	});

	test('a group page lists only its agents and opens their hub', async ({ page }) => {
		const local = await mockEnvironments(page);
		await page.goto(`/environments/${productionGroup.id}`);

		const main = page.locator('main');
		await expect(pageHeading(page, productionGroup.name)).toBeVisible();
		await expect(main.getByText(productionGroup.description, { exact: true })).toBeVisible();
		await expect(main.getByRole('link', { name: 'Environments', exact: true })).toHaveAttribute(
			'href',
			'/environments'
		);

		const cards = main.locator(`a[href^="/environments/${productionGroup.id}/"]`);
		await expect(cards).toHaveCount(2);
		for (const { href, name, status } of [
			{ href: remoteHub, name: remoteEnvironment.name, status: 'Online' },
			{ href: offlineHub, name: offlineEnvironment.name, status: 'Offline' }
		]) {
			const card = main.getByRole('link').filter({ hasText: name });
			await expect(card).toHaveAttribute('href', href);
			await expect(card).toContainText(status);
		}
		for (const name of [local.name, disabledEnvironment.name]) {
			await expect(main.getByRole('link').filter({ hasText: name })).toHaveCount(0);
		}

		await main.getByRole('link').filter({ hasText: remoteEnvironment.name }).click();
		await expect(page).toHaveURL((url) => url.pathname === remoteHub);
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();
		await expect(
			main.getByRole('link', { name: productionGroup.name, exact: true })
		).toHaveAttribute('href', `/environments/${productionGroup.id}`);
	});

	test('the Unassigned page lists the agents without a group', async ({ page }) => {
		const local = await mockEnvironments(page);
		await page.goto('/environments/unassigned');

		const main = page.locator('main');
		await expect(pageHeading(page, 'Unassigned')).toBeVisible();
		const cards = main.locator('a[href^="/environments/unassigned/"]');
		await expect(cards).toHaveCount(1);
		await expect(cards.first()).toHaveAttribute('href', localHub);
		await expect(cards.first()).toContainText(local.name);
	});

	test("a hub URL with the wrong group redirects to the agent's group", async ({ page }) => {
		await mockEnvironments(page);
		await mockRemoteNetworks(page);

		await page.goto(`/environments/unassigned/${remoteEnvironment.id}?tab=networks`);
		await expect(page).toHaveURL(
			(url) => url.pathname === remoteHub && url.searchParams.get('tab') === 'networks'
		);
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();
		await expect(environmentTab(page, 'Networks')).toHaveAttribute('data-state', 'active');
	});

	for (const { label, path } of [
		{ label: 'an unknown group', path: '/environments/e2e-missing-group' },
		{ label: 'a group without browsable agents', path: `/environments/${stagingGroup.id}` },
		{ label: 'an unknown agent', path: '/environments/unassigned/e2e-missing-environment' }
	]) {
		test(`${label} renders the not found page`, async ({ page }) => {
			await mockEnvironments(page);
			await page.goto(path);

			await expect(page.getByText(/not found - 404$/i)).toBeVisible();
		});
	}

	test('lists the containers the API reports for the environment without an environment column', async ({
		page
	}) => {
		const local = await mockEnvironments(page);
		const listed = await captureLocalContainers(page);
		const listPaths = recordContainerListPaths(page);

		await page.goto(localHub);
		await expect(pageHeading(page, local.name)).toBeVisible();
		await expect(
			page.getByText(`${localCounts.totalContainers} Total`, { exact: true })
		).toBeVisible();
		await expect(
			page.getByText(`${localCounts.runningContainers} Running`, { exact: true })
		).toBeVisible();
		await expect(
			page.getByText(`${localCounts.stoppedContainers} Stopped`, { exact: true })
		).toBeVisible();

		await expect
			.poll(() => listed.containers?.length ?? 0, { message: 'Local containers must be listed' })
			.toBeGreaterThan(0);
		const containers = listed.containers ?? [];
		const rows = resourceRows(page);
		await expect(rows).toHaveCount(containers.length);
		for (const container of containers) {
			await expect(
				rows.getByRole('link', { name: containerName(container), exact: true })
			).toHaveAttribute('href', `/containers/${container.id}?env=0`);
		}
		await expect(rows.getByRole('link', { name: containerName(remoteContainer) })).toHaveCount(0);
		await expectNoEnvironmentColumn(page);
		expect(listPaths).not.toEqual([]);
		expect(listPaths.every((pathname) => pathname === '/api/agents/0/containers')).toBe(true);
	});

	test('a remote environment page lists only that environment', async ({ page }) => {
		await mockEnvironments(page);
		const listPaths = recordContainerListPaths(page);

		await page.goto(remoteHub);
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();

		const rows = resourceRows(page);
		await expect(rows).toHaveCount(1);
		await expect(
			rows.getByRole('link', { name: containerName(remoteContainer), exact: true })
		).toHaveAttribute('href', `/containers/${remoteContainer.id}?env=${remoteEnvironment.id}`);
		await expectNoEnvironmentColumn(page);
		expect(listPaths).not.toEqual([]);
		expect(
			listPaths.every((pathname) => pathname === `/api/agents/${remoteEnvironment.id}/containers`)
		).toBe(true);
	});

	test('container names open the container details for the environment', async ({ page }) => {
		await mockEnvironments(page);
		const listed = await captureLocalContainers(page);

		await page.goto(localHub);
		await expect
			.poll(() => listed.containers?.length ?? 0, { message: 'Local containers must be listed' })
			.toBeGreaterThan(0);
		const [container] = listed.containers ?? [];
		await resourceRows(page)
			.getByRole('link', { name: containerName(container), exact: true })
			.click();

		await expect(page).toHaveURL(
			(url) => url.pathname === `/containers/${container.id}` && url.searchParams.get('env') === '0'
		);
		await expect(page.getByRole('tab', { name: 'Overview', exact: true })).toBeVisible();
	});

	test('shows a tab per resource and keeps the active tab in the URL', async ({ page }) => {
		await mockEnvironments(page);
		await mockRemoteNetworks(page);

		await page.goto(remoteHub);
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();
		for (const name of ['Containers', 'Projects', 'Images', 'Volumes', 'Networks']) {
			await expect(environmentTab(page, name)).toBeVisible();
		}
		await expect(environmentTab(page, 'Containers')).toHaveAttribute('data-state', 'active');
		await expect(page).toHaveURL(hasTabParam('containers'));

		await environmentTab(page, 'Networks').click();
		await expect(page).toHaveURL(hasTabParam('networks'));
		await expect(environmentTab(page, 'Networks')).toHaveAttribute('data-state', 'active');
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();
		await expect(
			resourceRows(page).getByRole('link', { name: remoteNetwork.name, exact: true })
		).toBeVisible();

		await page.goto(`${remoteHub}?tab=networks`);
		await expect(environmentTab(page, 'Networks')).toHaveAttribute('data-state', 'active');

		await page.goto(`${remoteHub}?tab=unknown`);
		await expect(environmentTab(page, 'Containers')).toHaveAttribute('data-state', 'active');
		await expect(page).toHaveURL(hasTabParam('containers'));
	});

	test('shows only the tabs the user can list in the environment', async ({ page }) => {
		const local = await mockEnvironments(page);
		await mockUserPermissions(page, [local, ...remoteEnvironments], {
			global: [],
			[remoteEnvironment.id]: ['networks:list']
		});
		await mockRemoteNetworks(page);

		await page.goto(`${remoteHub}?tab=containers`);
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();
		await expect(environmentTab(page, 'Networks')).toHaveAttribute('data-state', 'active');
		await expect(page).toHaveURL(hasTabParam('networks'));
		for (const name of ['Containers', 'Projects', 'Images', 'Volumes']) {
			await expect(environmentTab(page, name)).toHaveCount(0);
		}
		await expect(resourceRows(page)).toHaveCount(1);
	});

	test('the Networks tab lists only that environment and loads no other tab', async ({ page }) => {
		await mockEnvironments(page);
		await mockRemoteNetworks(page);
		const containerPaths = recordContainerListPaths(page);
		const networkPaths: string[] = [];
		page.on('request', (request) => {
			const { pathname } = new URL(request.url());
			if (/^\/api\/agents\/[^/]+\/networks$/.test(pathname)) networkPaths.push(pathname);
		});

		await page.goto(`${remoteHub}?tab=networks`);
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();
		await expect(page.getByText('1 Total Network', { exact: true })).toBeVisible();

		const rows = resourceRows(page);
		await expect(rows).toHaveCount(1);
		await expect(rows.getByRole('link', { name: remoteNetwork.name, exact: true })).toHaveAttribute(
			'href',
			`/networks/${remoteNetwork.id}?env=${remoteEnvironment.id}`
		);
		await expectNoEnvironmentColumn(page);
		expect(networkPaths).not.toEqual([]);
		expect(
			networkPaths.every((pathname) => pathname === `/api/agents/${remoteEnvironment.id}/networks`)
		).toBe(true);
		expect(containerPaths).toEqual([]);
	});

	test('an offline environment shows when it was last seen and loads on Retry', async ({
		page,
		pageErrorGuard
	}) => {
		pageErrorGuard.allow('agent tunnel not connected');
		await mockEnvironments(page);
		const containers = await mockUnreachableContainers(
			page,
			offlineEnvironment.id,
			'agent tunnel not connected'
		);
		const listPaths = recordContainerListPaths(page);
		const offlineTitle = page.getByText(`${offlineEnvironment.name} is offline`, { exact: true });
		const retry = page.getByRole('button', { name: 'Retry', exact: true });

		await page.goto(offlineHub);
		await expect(pageHeading(page, offlineEnvironment.name)).toBeVisible();
		await expect(offlineTitle).toBeVisible();
		await expect(page.getByText(/^Last seen /)).toBeVisible();
		expect(listPaths).toEqual([]);

		await retry.click();
		await expect(page.getByText(/agent tunnel not connected/)).toBeVisible({ timeout: 15_000 });
		await expect(offlineTitle).toBeVisible();

		containers.reachable = true;
		await retry.click();
		await expect(offlineTitle).toHaveCount(0, { timeout: 15_000 });
		await expect(page.getByText('0 Total', { exact: true })).toBeVisible();
		expect(listPaths).not.toEqual([]);
		expect(
			listPaths.every((pathname) => pathname === `/api/agents/${offlineEnvironment.id}/containers`)
		).toBe(true);
	});

	test('an unreachable environment renders inline instead of the error page', async ({
		page,
		pageErrorGuard
	}) => {
		pageErrorGuard.allow('remote unavailable');
		await mockEnvironments(page);
		const containers = await mockUnreachableContainers(
			page,
			remoteEnvironment.id,
			'remote unavailable'
		);

		await page.goto(remoteHub);
		await expect(pageHeading(page, remoteEnvironment.name)).toBeVisible();
		await expect(
			page.getByText(`${remoteEnvironment.name} is unreachable`, { exact: true })
		).toBeVisible({ timeout: 15_000 });
		await expect(page.getByText(/remote unavailable/)).toBeVisible();

		containers.reachable = true;
		await page.getByRole('button', { name: 'Retry', exact: true }).click();
		await expect(
			resourceRows(page).getByRole('link', { name: containerName(remoteContainer), exact: true })
		).toBeVisible({ timeout: 15_000 });
	});
});

test.describe('Environment access', () => {
	const accessPath = `/api/environment-groups/${productionGroup.id}/role-assignments`;
	const timestamp = '2026-01-01T00:00:00Z';
	const users = [
		{ id: 'e2e-user-alice', username: 'e2e-alice', displayName: 'Alice Example' },
		{ id: 'e2e-user-carol', username: 'e2e-carol', displayName: 'Carol Example' }
	];
	const roles = [
		{ id: 'role_editor', name: 'Editor', description: 'Manage resources' },
		{ id: 'role_viewer', name: 'Viewer', description: 'Read-only access' }
	].map((role) => ({
		...role,
		permissions: [],
		builtIn: true,
		assignedUserCount: 0,
		createdAt: timestamp
	}));
	const platformUserGroup = {
		id: 'e2e-user-group-platform',
		name: 'Platform',
		description: 'Platform engineers',
		memberCount: 2,
		links: [],
		createdAt: timestamp,
		updatedAt: timestamp
	};

	type AccessEntry = {
		id: string;
		principalType: 'user' | 'group';
		userId?: string;
		username?: string;
		displayName?: string;
		userGroupId?: string;
		userGroupName?: string;
		roleId: string;
		roleName: string;
		source: 'manual' | 'oidc' | 'ldap';
	};

	// Serves the group's access list, recording grants and removals, beside mocked users, user groups and roles.
	async function mockAccess(page: Page) {
		const entries: AccessEntry[] = [
			{
				id: 'e2e-assignment-sso',
				principalType: 'user',
				userId: 'e2e-user-bob',
				username: 'e2e-bob',
				roleId: 'role_editor',
				roleName: 'Editor',
				source: 'oidc'
			},
			{
				id: 'e2e-assignment-manual',
				principalType: 'user',
				userId: 'e2e-user-carol',
				username: 'e2e-carol',
				displayName: 'Carol Example',
				roleId: 'role_viewer',
				roleName: 'Viewer',
				source: 'manual'
			}
		];
		const granted: unknown[] = [];
		const removed: string[] = [];
		await page.route(/\/api\/environment-groups(?:\?.*)?$/, (route) =>
			route.fulfill({ json: { success: true, data: [groupDto(productionGroup, 0)] } })
		);
		await page.route(/\/api\/users(?:\?.*)?$/, (route) =>
			route.fulfill({ json: paginated(users) })
		);
		await page.route(/\/api\/roles(?:\?.*)?$/, (route) =>
			route.fulfill({ json: paginated(roles) })
		);
		await page.route(/\/api\/user-groups(?:\?.*)?$/, (route) =>
			route.fulfill({ json: { success: true, data: [platformUserGroup] } })
		);
		await page.route(
			(url) => url.pathname === accessPath,
			async (route) => {
				if (route.request().method() !== 'POST') {
					await route.fulfill({ json: { success: true, data: entries } });
					return;
				}
				const body = route.request().postDataJSON() as {
					userId?: string;
					userGroupId?: string;
					roleId: string;
				};
				granted.push(body);
				const role = roles.find((candidate) => candidate.id === body.roleId);
				const grant = {
					id: 'e2e-assignment-granted',
					roleId: body.roleId,
					roleName: role?.name ?? body.roleId,
					source: 'manual' as const
				};
				let entry: AccessEntry;
				if (body.userGroupId) {
					entry = {
						...grant,
						principalType: 'group',
						userGroupId: body.userGroupId,
						userGroupName: platformUserGroup.name
					};
				} else {
					const user = users.find((candidate) => candidate.id === body.userId);
					entry = {
						...grant,
						principalType: 'user',
						userId: body.userId,
						username: user?.username ?? body.userId,
						displayName: user?.displayName
					};
				}
				entries.push(entry);
				await route.fulfill({ status: 201, json: { success: true, data: entry } });
			}
		);
		await page.route(
			(url) => url.pathname.startsWith(`${accessPath}/`),
			async (route) => {
				const assignmentId = new URL(route.request().url()).pathname.slice(accessPath.length + 1);
				removed.push(assignmentId);
				entries.splice(
					entries.findIndex((entry) => entry.id === assignmentId),
					1
				);
				await route.fulfill({ json: { success: true } });
			}
		);
		return { granted, removed };
	}

	test('the Access sheet lists, grants and removes access to the environment', async ({ page }) => {
		const { granted, removed } = await mockAccess(page);
		await page.goto(`/environments/${productionGroup.id}`);

		await page.getByRole('button', { name: 'Access', exact: true }).click();
		const sheet = page.getByRole('dialog', { name: 'Access', exact: true });
		await waitForDialogReady(sheet);

		const ssoRow = sheet.getByRole('row').filter({ hasText: 'e2e-bob' });
		await expect(ssoRow).toContainText('Editor');
		await expect(ssoRow).toContainText('SSO');
		await expect(ssoRow.getByRole('button', { name: 'Remove', exact: true })).toHaveCount(0);
		const manualRow = sheet.getByRole('row').filter({ hasText: 'e2e-carol' });
		await expect(manualRow).toContainText('Viewer');
		await expect(manualRow).toContainText('Manual');

		await sheet.getByRole('combobox', { name: 'User', exact: true }).click();
		await page.getByRole('option').filter({ hasText: 'e2e-alice' }).click();
		await sheet.getByRole('combobox', { name: 'Role', exact: true }).click();
		await page.getByRole('option').filter({ hasText: 'Editor' }).click();
		await sheet.getByRole('button', { name: 'Grant Access', exact: true }).click();

		await expect.poll(() => granted).toEqual([{ userId: 'e2e-user-alice', roleId: 'role_editor' }]);
		const grantedRow = sheet.getByRole('row').filter({ hasText: 'e2e-alice' });
		await expect(grantedRow).toContainText('Editor');
		await expect(grantedRow).toContainText('Manual');

		await manualRow.getByRole('button', { name: 'Remove', exact: true }).click();
		const confirm = page.getByRole('dialog', { name: 'Remove Access', exact: true });
		await expect(confirm).toContainText(
			`Remove the Viewer role from e2e-carol on ${productionGroup.name}?`
		);
		await confirm.getByRole('button', { name: 'Remove', exact: true }).click();

		await expect.poll(() => removed).toEqual(['e2e-assignment-manual']);
		await expect(manualRow).toHaveCount(0);
		await expect(ssoRow).toBeVisible();
	});

	test('the Access sheet grants a role to a user group and removes it', async ({ page }) => {
		const { granted, removed } = await mockAccess(page);
		await page.goto(`/environments/${productionGroup.id}`);

		await page.getByRole('button', { name: 'Access', exact: true }).click();
		const sheet = page.getByRole('dialog', { name: 'Access', exact: true });
		await waitForDialogReady(sheet);

		await sheet.getByRole('button', { name: 'User Group', exact: true }).click();
		await sheet.getByRole('combobox', { name: 'User Group', exact: true }).click();
		await page.getByRole('option').filter({ hasText: platformUserGroup.name }).click();
		await sheet.getByRole('combobox', { name: 'Role', exact: true }).click();
		await page.getByRole('option').filter({ hasText: 'Viewer' }).click();
		await sheet.getByRole('button', { name: 'Grant Access', exact: true }).click();

		await expect
			.poll(() => granted)
			.toEqual([{ userGroupId: platformUserGroup.id, roleId: 'role_viewer' }]);
		const groupRow = sheet.getByRole('row').filter({ hasText: platformUserGroup.name });
		await expect(groupRow).toContainText('Viewer');
		await expect(groupRow).toContainText('Manual');

		await groupRow.getByRole('button', { name: 'Remove', exact: true }).click();
		const confirm = page.getByRole('dialog', { name: 'Remove Access', exact: true });
		await expect(confirm).toContainText(
			`Remove the Viewer role from ${platformUserGroup.name} on ${productionGroup.name}?`
		);
		await confirm.getByRole('button', { name: 'Remove', exact: true }).click();

		await expect.poll(() => removed).toEqual(['e2e-assignment-granted']);
		await expect(groupRow).toHaveCount(0);
	});
});
