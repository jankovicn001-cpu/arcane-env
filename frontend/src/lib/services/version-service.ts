import { version as currentVersion } from '$app/env';

import type { VersionInfo } from '#lib/types/version.js';
import { tryCatch } from '#lib/utils/try-catch.js';

import { apiClient } from './api-service';

function getCurrentVersion() {
	return currentVersion;
}

export function toAppVersionInformation(data: Partial<VersionInfo>): VersionInfo {
	return {
		currentVersion: data.currentVersion || getCurrentVersion(),
		currentTag: data.currentTag,
		currentDigest: data.currentDigest,
		displayVersion: data.displayVersion || data.currentVersion || getCurrentVersion(),
		revision: data.revision || 'unknown',
		shortRevision: data.shortRevision || data.revision?.slice(0, 8) || 'unknown',
		goVersion: data.goVersion || 'unknown',
		nodeVersion: data.nodeVersion || 'unknown',
		svelteKitVersion: data.svelteKitVersion || 'unknown',
		enabledFeatures: data.enabledFeatures ?? [],
		buildTime: data.buildTime,
		isSemverVersion: data.isSemverVersion || false,
		isStableVersion: data.isStableVersion || false
	};
}

async function getVersionInformation(): Promise<VersionInfo> {
	const response = await tryCatch(apiClient.get<Partial<VersionInfo>>('/app/version', { timeout: 2000 }));
	if (response.error !== null) return toAppVersionInformation({});
	return toAppVersionInformation(response.data.data);
}

export default {
	getVersionInformation,
	getCurrentVersion
};
