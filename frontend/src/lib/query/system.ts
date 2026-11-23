import { queryOptions } from '@tanstack/svelte-query';

import { queryKeys } from '#lib/query/query-keys.js';
import { environmentManagementService } from '#lib/services/env-mgmt-service.js';
import { systemService } from '#lib/services/system-service.js';
import versionService from '#lib/services/version-service.js';

export function appVersionQuery() {
	return queryOptions({
		queryKey: queryKeys.system.appVersion(),
		queryFn: () => versionService.getVersionInformation()
	});
}

export function environmentVersionInfoQuery(environmentId: string) {
	return queryOptions({
		queryKey: queryKeys.system.versionInfo(environmentId),
		queryFn: () => environmentManagementService.getVersion(environmentId)
	});
}

export function dockerInfoQuery(environmentId: string) {
	return queryOptions({
		queryKey: queryKeys.system.dockerInfo(environmentId),
		queryFn: () => systemService.getDockerInfoForEnvironment(environmentId)
	});
}
