export type WebhookTargetType = 'container' | 'project' | 'gitops';
export type WebhookActionType = 'start' | 'stop' | 'restart' | 'redeploy' | 'up' | 'down' | 'sync';

export type Webhook = {
	id: string;
	name: string;
	tokenPrefix: string;
	targetType: WebhookTargetType;
	actionType: WebhookActionType;
	targetId: string;
	targetName?: string;
	environmentId?: string;
	environmentName?: string;
	enabled: boolean;
	lastTriggeredAt?: string;
	createdAt: string;
};

export type WebhookCreated = {
	id: string;
	name: string;
	token: string;
	targetType: WebhookTargetType;
	actionType: WebhookActionType;
	targetId: string;
	createdAt: string;
};
