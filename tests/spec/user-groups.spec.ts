import { waitForDialogReady } from '../utils/playwright.util';
import { test, expect, type Page, type Route } from '../fixtures/test.fixture';

type UserGroupLink = { kind: 'oidc' | 'ldap'; value: string };

type UserGroupDto = {
	id: string;
	name: string;
	description: string;
	memberCount: number;
	links: UserGroupLink[];
	createdAt: string;
	updatedAt: string;
};

type MemberDto = {
	id: string;
	userId: string;
	username: string;
	displayName?: string;
	source: 'manual' | 'oidc' | 'ldap';
};

type AssignmentDto = {
	id: string;
	roleId: string;
	roleName: string;
	environmentId?: string;
	environmentGroupId?: string;
};

const timestamp = '2026-01-01T00:00:00Z';
const userGroupsPath = '/api/user-groups';

const users = [
	{ id: 'e2e-user-alice', username: 'e2e-alice', displayName: 'Alice Example' },
	{ id: 'e2e-user-bob', username: 'e2e-bob', displayName: 'Bob Example' },
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

const productionEnvironment = {
	id: 'e2e-group-production',
	name: 'Production',
	description: '',
	environmentCount: 0,
	createdAt: timestamp,
	updatedAt: timestamp
};

const platformGroup: UserGroupDto = {
	id: 'e2e-user-group-platform',
	name: 'Platform',
	description: 'Platform engineers',
	memberCount: 2,
	links: [{ kind: 'oidc', value: 'platform' }],
	createdAt: timestamp,
	updatedAt: timestamp
};

function paginated<T>(data: T[]) {
	return {
		success: true,
		data,
		pagination: {
			totalPages: 1,
			totalItems: data.length,
			currentPage: 1,
			itemsPerPage: 20,
			grandTotalItems: data.length
		}
	};
}

// Serves the user groups API from memory, recording every write, beside mocked users, roles and environments.
async function mockUserGroups(page: Page) {
	const groups: UserGroupDto[] = [structuredClone(platformGroup)];
	const members: MemberDto[] = [
		{
			id: 'e2e-member-bob',
			userId: 'e2e-user-bob',
			username: 'e2e-bob',
			displayName: 'Bob Example',
			source: 'oidc'
		},
		{
			id: 'e2e-member-carol',
			userId: 'e2e-user-carol',
			username: 'e2e-carol',
			displayName: 'Carol Example',
			source: 'manual'
		}
	];
	const assignments: AssignmentDto[] = [
		{ id: 'e2e-group-assignment-viewer', roleId: 'role_viewer', roleName: 'Viewer' }
	];
	const writes = {
		created: [] as unknown[],
		updated: [] as Array<{ id: string; body: unknown }>,
		deleted: [] as string[],
		addedMembers: [] as unknown[],
		removedMembers: [] as string[],
		granted: [] as unknown[],
		revoked: [] as string[]
	};

	await page.route(/\/api\/users(?:\?.*)?$/, (route) => route.fulfill({ json: paginated(users) }));
	await page.route(/\/api\/roles(?:\?.*)?$/, (route) => route.fulfill({ json: paginated(roles) }));
	await page.route(/\/api\/environment-groups(?:\?.*)?$/, (route) =>
		route.fulfill({ json: { success: true, data: [productionEnvironment] } })
	);

	async function handle(route: Route) {
		const request = route.request();
		const method = request.method();
		const [groupId, child, childId] = new URL(request.url()).pathname
			.slice(userGroupsPath.length + 1)
			.split('/');
		const ok = (data: unknown, status = 200) =>
			route.fulfill({ status, json: { success: true, data } });

		if (!groupId) {
			if (method === 'POST') {
				const body = request.postDataJSON() as Pick<UserGroupDto, 'name' | 'description' | 'links'>;
				writes.created.push(body);
				const created: UserGroupDto = {
					...body,
					id: `e2e-user-group-${groups.length + 1}`,
					memberCount: 0,
					createdAt: timestamp,
					updatedAt: timestamp
				};
				groups.push(created);
				await ok(created, 201);
				return;
			}
			await ok(groups);
			return;
		}

		const index = groups.findIndex((group) => group.id === groupId);
		if (!child) {
			if (method === 'PATCH') {
				const body = request.postDataJSON() as Partial<UserGroupDto>;
				writes.updated.push({ id: groupId, body });
				groups[index] = { ...groups[index], ...body };
				await ok(groups[index]);
				return;
			}
			if (method === 'DELETE') {
				writes.deleted.push(groupId);
				groups.splice(index, 1);
				await ok(undefined);
				return;
			}
			await ok(groups[index]);
			return;
		}

		if (child === 'members') {
			if (method === 'POST') {
				const body = request.postDataJSON() as { userId: string };
				writes.addedMembers.push(body);
				const user = users.find((candidate) => candidate.id === body.userId);
				const member: MemberDto = {
					id: `e2e-member-${body.userId}`,
					userId: body.userId,
					username: user?.username ?? body.userId,
					displayName: user?.displayName,
					source: 'manual'
				};
				members.push(member);
				await ok(member, 201);
				return;
			}
			if (method === 'DELETE' && childId) {
				writes.removedMembers.push(childId);
				members.splice(
					members.findIndex((member) => member.id === childId),
					1
				);
				await ok(undefined);
				return;
			}
			await ok(members);
			return;
		}

		if (method === 'POST') {
			const body = request.postDataJSON() as Omit<AssignmentDto, 'id' | 'roleName'>;
			writes.granted.push(body);
			const assignment: AssignmentDto = {
				...body,
				id: 'e2e-group-assignment-granted',
				roleName: roles.find((role) => role.id === body.roleId)?.name ?? body.roleId
			};
			assignments.push(assignment);
			await ok(assignment, 201);
			return;
		}
		if (method === 'DELETE' && childId) {
			writes.revoked.push(childId);
			assignments.splice(
				assignments.findIndex((assignment) => assignment.id === childId),
				1
			);
			await ok(undefined);
			return;
		}
		await ok(assignments);
	}

	await page.route(
		(url) => url.pathname === userGroupsPath || url.pathname.startsWith(`${userGroupsPath}/`),
		handle
	);
	return writes;
}

test.describe('User groups', () => {
	test('the User Groups page creates a group with linked values, edits it and deletes it', async ({
		page
	}) => {
		const writes = await mockUserGroups(page);
		await page.goto('/system/user-groups');

		const platformRow = page.getByRole('row').filter({ hasText: platformGroup.name });
		await expect(platformRow).toContainText('Platform engineers');
		await expect(platformRow).toContainText('SSO: platform');
		await expect(platformRow.getByRole('cell').nth(2)).toHaveText('2');

		await page.getByRole('button', { name: 'Create User Group', exact: true }).click();
		const createSheet = page.getByRole('dialog', { name: 'Create User Group', exact: true });
		await waitForDialogReady(createSheet);
		await createSheet.getByLabel('Name', { exact: true }).fill('Operations');
		await createSheet.getByLabel('Description', { exact: true }).fill('Ops team');
		await createSheet.getByRole('button', { name: 'Add Linked Group', exact: true }).click();
		await createSheet.getByRole('combobox', { name: 'Type', exact: true }).click();
		await page.getByRole('option', { name: 'LDAP', exact: true }).click();
		await createSheet.getByRole('textbox', { name: 'Value', exact: true }).fill(' cn=ops ');
		await createSheet.getByRole('button', { name: 'Create', exact: true }).click();

		await expect
			.poll(() => writes.created)
			.toEqual([
				{ name: 'Operations', description: 'Ops team', links: [{ kind: 'ldap', value: 'cn=ops' }] }
			]);
		await expect(createSheet).toBeHidden();
		const operationsRow = page.getByRole('row').filter({ hasText: 'Operations' });
		await expect(operationsRow).toContainText('LDAP: cn=ops');

		await operationsRow.getByRole('button', { name: 'Edit', exact: true }).click();
		const editSheet = page.getByRole('dialog', { name: 'Edit User Group', exact: true });
		await waitForDialogReady(editSheet);
		await expect(editSheet.getByLabel('Name', { exact: true })).toHaveValue('Operations');
		await editSheet.getByLabel('Description', { exact: true }).fill('Operations team');
		await editSheet.getByRole('button', { name: 'Save', exact: true }).click();

		// Only changed fields are sent, so the links are left alone.
		await expect
			.poll(() => writes.updated)
			.toEqual([{ id: 'e2e-user-group-2', body: { description: 'Operations team' } }]);
		await expect(operationsRow).toContainText('Operations team');

		await operationsRow.getByRole('button', { name: 'Delete', exact: true }).click();
		const confirm = page.getByRole('dialog', { name: 'Delete Operations', exact: true });
		await confirm.getByRole('button', { name: 'Delete', exact: true }).click();

		await expect.poll(() => writes.deleted).toEqual(['e2e-user-group-2']);
		await expect(operationsRow).toHaveCount(0);
		await expect(platformRow).toBeVisible();
	});

	test('the Members tab adds manual members and keeps synced members read-only', async ({
		page
	}) => {
		const writes = await mockUserGroups(page);
		await page.goto(`/system/user-groups/${platformGroup.id}`);

		const ssoRow = page.getByRole('row').filter({ hasText: 'e2e-bob' });
		await expect(ssoRow).toContainText('SSO');
		await expect(ssoRow.getByRole('button', { name: 'Remove', exact: true })).toHaveCount(0);
		const manualRow = page.getByRole('row').filter({ hasText: 'e2e-carol' });
		await expect(manualRow).toContainText('Manual');

		await page.getByRole('combobox', { name: 'User', exact: true }).click();
		await page.getByRole('option').filter({ hasText: 'e2e-alice' }).click();
		await page.getByRole('button', { name: 'Add Member', exact: true }).click();

		await expect.poll(() => writes.addedMembers).toEqual([{ userId: 'e2e-user-alice' }]);
		const addedRow = page.getByRole('row').filter({ hasText: 'e2e-alice' });
		await expect(addedRow).toContainText('Manual');

		await manualRow.getByRole('button', { name: 'Remove', exact: true }).click();
		const confirm = page.getByRole('dialog', { name: 'Remove Member', exact: true });
		await expect(confirm).toContainText('Remove e2e-carol from this group?');
		await confirm.getByRole('button', { name: 'Remove', exact: true }).click();

		await expect.poll(() => writes.removedMembers).toEqual(['e2e-member-carol']);
		await expect(manualRow).toHaveCount(0);
		await expect(ssoRow).toBeVisible();
	});

	test('the Access tab grants a role on an environment and removes a grant', async ({ page }) => {
		const writes = await mockUserGroups(page);
		await page.goto(`/system/user-groups/${platformGroup.id}?tab=access`);

		const globalRow = page.getByRole('row').filter({ hasText: 'Viewer' });
		await expect(globalRow).toContainText('Global (org-wide)');

		await page.getByRole('combobox', { name: 'Role', exact: true }).click();
		await page.getByRole('option').filter({ hasText: 'Editor' }).click();
		await page.getByRole('combobox', { name: 'Scope', exact: true }).click();
		await page.getByRole('option', { name: 'Environment: Production', exact: true }).click();
		await page.getByRole('button', { name: 'Grant Access', exact: true }).click();

		await expect
			.poll(() => writes.granted)
			.toEqual([{ roleId: 'role_editor', environmentGroupId: productionEnvironment.id }]);
		const grantedRow = page.getByRole('row').filter({ hasText: 'Editor' });
		await expect(grantedRow).toContainText('Environment: Production');

		await globalRow.getByRole('button', { name: 'Remove', exact: true }).click();
		const confirm = page.getByRole('dialog', { name: 'Remove Access', exact: true });
		await expect(confirm).toContainText(
			'Remove the Viewer role on Global (org-wide) from this group?'
		);
		await confirm.getByRole('button', { name: 'Remove', exact: true }).click();

		await expect.poll(() => writes.revoked).toEqual(['e2e-group-assignment-viewer']);
		await expect(globalRow).toHaveCount(0);
		await expect(grantedRow).toBeVisible();
	});
});
