import type { Environment, EnvironmentGroup } from '../environment';
import type { CreateRequest, UpdateRequest } from './pagination';

export type EnvironmentWritable = 'name' | 'groupId' | 'enabled';

export type EnvironmentCreate = CreateRequest<Environment, EnvironmentWritable, 'name'>;

export type EnvironmentUpdate = UpdateRequest<Environment, EnvironmentWritable>;

export type EnvironmentGroupCreate = CreateRequest<EnvironmentGroup, 'name' | 'description', 'name'>;

export type EnvironmentGroupUpdate = UpdateRequest<EnvironmentGroup, 'name' | 'description'>;
