import { m } from '#lib/paraglide/messages.js';

type NamedOption = { id: string; name: string };

/** Where a role assignment applies; at most one of the two ids is set, and neither means global. */
export type RoleAssignmentScope = { environmentId?: string | null; environmentGroupId?: string | null };

function selectedOptionLabel<T extends NamedOption>(options: T[], value: string, fallback: string): string {
	return options.find((option) => option.id === value)?.name ?? fallback;
}

const nameCollator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

function byName(a: NamedOption, b: NamedOption): number {
	return nameCollator.compare(a.name, b.name);
}

export const GLOBAL_ENVIRONMENT_OPTION_ID = 'global';
const ENVIRONMENT_GROUP_OPTION_PREFIX = 'group:';

/** The scope picker's option id for a scope. */
export function roleScopeOptionId({ environmentId, environmentGroupId }: RoleAssignmentScope): string {
	if (environmentGroupId) return `${ENVIRONMENT_GROUP_OPTION_PREFIX}${environmentGroupId}`;
	return environmentId || GLOBAL_ENVIRONMENT_OPTION_ID;
}

/** The scope a picker option selects; picking one kind clears the other. */
export function roleScopeFromOptionId(optionId: string): { environmentId?: string; environmentGroupId?: string } {
	if (optionId === GLOBAL_ENVIRONMENT_OPTION_ID) return { environmentId: undefined, environmentGroupId: undefined };
	if (optionId.startsWith(ENVIRONMENT_GROUP_OPTION_PREFIX)) {
		return { environmentId: undefined, environmentGroupId: optionId.slice(ENVIRONMENT_GROUP_OPTION_PREFIX.length) };
	}
	return { environmentId: optionId, environmentGroupId: undefined };
}

/** Scope options in picker order: Global, then environments, then agents. */
export function buildRoleScopeOptions(environments: NamedOption[], groups: NamedOption[]): NamedOption[] {
	return [
		{ id: GLOBAL_ENVIRONMENT_OPTION_ID, name: m.global_org_wide() },
		...groups.toSorted(byName).map((group) => ({
			id: roleScopeOptionId({ environmentGroupId: group.id }),
			name: m.environment_name({ name: group.name })
		})),
		...environments.toSorted(byName).map((env) => ({ id: env.id, name: m.account_role_environment({ env: env.name }) }))
	];
}

/** Display label for an assignment's scope; unknown ids show as is. */
export function roleScopeLabel(scope: RoleAssignmentScope, environments: NamedOption[], groups: NamedOption[]): string {
	const { environmentId, environmentGroupId } = scope;
	if (environmentGroupId) {
		return m.environment_name({ name: groups.find((group) => group.id === environmentGroupId)?.name ?? environmentGroupId });
	}
	if (environmentId) {
		return m.account_role_environment({ env: environments.find((env) => env.id === environmentId)?.name ?? environmentId });
	}
	return m.global_org_wide();
}

export function createRoleEnvironmentLabelers<
	TRole extends { id: string; name: string },
	TEnvironment extends { id: string; name: string }
>(roles: TRole[], environments: TEnvironment[], fallback: string) {
	return {
		environment: (value: string) => selectedOptionLabel(environments, value, fallback),
		role: (value: string) => selectedOptionLabel(roles, value, fallback)
	};
}
