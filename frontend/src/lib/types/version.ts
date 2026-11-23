export type VersionInfo = {
	currentVersion: string;
	currentTag?: string;
	currentDigest?: string;
	displayVersion: string;
	revision: string;
	shortRevision: string;
	goVersion: string;
	nodeVersion: string;
	svelteKitVersion: string;
	enabledFeatures?: string[];
	buildTime?: string;
	isSemverVersion: boolean;
	isStableVersion: boolean;
};
