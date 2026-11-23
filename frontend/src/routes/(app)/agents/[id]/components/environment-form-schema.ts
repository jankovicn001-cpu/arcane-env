import { z } from 'zod/v4';

export const environmentUpdateSchema = z.object({
	name: z.string().min(1),
	/** The environment group's id; empty for none. */
	groupId: z.string(),
	enabled: z.boolean()
});

export const environmentFormSchema = environmentUpdateSchema.extend({
	featureVulnerabilityManagementEnabled: z.boolean(),
	featureSwarmEnabled: z.boolean()
});

export type EnvironmentFormValues = z.infer<typeof environmentFormSchema>;
