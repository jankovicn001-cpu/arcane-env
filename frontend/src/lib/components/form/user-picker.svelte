<script lang="ts">
	import { createQuery, keepPreviousData } from '@tanstack/svelte-query';

	import SearchableSelect from '#lib/components/form/searchable-select.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { userListQuery } from '#lib/query/users.js';
	import { extractApiErrorMessage } from '#lib/utils/api.js';

	let {
		value = $bindable(''),
		triggerId,
		disabled = false
	}: {
		value?: string;
		triggerId: string;
		disabled?: boolean;
	} = $props();

	// The server searches, so only the first page of matches is listed.
	const USER_PICKER_LIMIT = 50;

	let search = $state('');
	let username = $state('');

	const usersQuery = createQuery(() => ({
		...userListQuery({
			search: search.trim() || undefined,
			pagination: { page: 1, limit: USER_PICKER_LIMIT },
			sort: { column: 'Username', direction: 'asc' }
		}),
		placeholderData: keepPreviousData
	}));
	const items = $derived(
		(usersQuery.data?.data ?? []).map((user) => ({ value: user.id, label: user.username, hint: user.displayName ?? undefined }))
	);

	// The chosen user may drop out of later searches, so its name is kept.
	function select(id: string) {
		username = items.find((item) => item.value === id)?.label ?? id;
		value = id;
		search = '';
	}
</script>

<SearchableSelect
	{triggerId}
	class="w-full"
	size="default"
	{items}
	bind:value={() => value, select}
	displayText={value ? username : undefined}
	oninput={(event) => (search = event.currentTarget.value)}
	isLoading={usersQuery.isFetching}
	emptyText={usersQuery.error ? extractApiErrorMessage(usersQuery.error) : m.common_no_results_found()}
	{disabled}
/>
