import type { SearchPaginationSortRequest } from '#lib/types/ui/pagination.js';

export type ActionStatus =
	| 'starting'
	| 'stopping'
	| 'restarting'
	| 'redeploying'
	| 'archiving'
	| 'unarchiving'
	| 'destroying'
	| 'syncing'
	| '';

export function withArchivedFilter(options: SearchPaginationSortRequest, show: boolean): SearchPaginationSortRequest {
	const filters = { ...options.filters };
	if (show) {
		filters['archived'] = 'true';
	} else {
		delete filters['archived'];
	}

	return {
		...options,
		filters: Object.keys(filters).length > 0 ? filters : undefined
	};
}
