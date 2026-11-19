import type {
  Dashboard,
  Device,
  DeviceWithToken,
  ProfileUpdate,
  Project,
  ProjectInput,
  Submission,
  User,
} from './types';

const API_ROOT = '/api/v1';

export class ApiError extends Error {
  status: number;

  constructor(message: string, status = 0) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

function errorMessage(body: unknown, status: number): string {
  if (body && typeof body === 'object') {
    const value = body as { error?: unknown; message?: unknown };
    if (typeof value.error === 'string') return value.error;
    if (typeof value.message === 'string') return value.message;
  }
  if (status === 401) return 'Your session has expired. Please sign in again.';
  if (status === 404) return 'We could not find that page or resource.';
  if (status >= 500) return 'The server is having trouble. Please try again.';
  return `Request failed (${status}). Please try again.`;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_ROOT}${path}`, {
      ...init,
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        ...(init.body ? { 'Content-Type': 'application/json' } : {}),
        ...init.headers,
      },
    });
  } catch {
    throw new ApiError('Cannot reach GritLoom. Check your connection or local server and try again.');
  }

  const text = await response.text();
  let body: unknown = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      if (!response.ok) throw new ApiError(errorMessage(null, response.status), response.status);
      throw new ApiError('The server returned an invalid response. Please try again.');
    }
  }

  if (!response.ok) throw new ApiError(errorMessage(body, response.status), response.status);
  return body as T;
}

const json = (method: string, body?: unknown): RequestInit => ({
  method,
  ...(body === undefined ? {} : { body: JSON.stringify(body) }),
});

export const api = {
  login: (username: string, password: string) =>
    request<User>('/auth/login', json('POST', { username, password })),
  register: (username: string, displayName: string, password: string) =>
    request<User>('/auth/register', json('POST', { username, display_name: displayName, password })),
  logout: () => request<null>('/auth/logout', json('POST')),
  dashboard: () => request<Dashboard>('/dashboard'),
  publicProfile: (username: string) => request<Dashboard>(`/u/${encodeURIComponent(username)}`),
  patchMe: (input: ProfileUpdate) => request<User>('/me', json('PATCH', input)),
  createProject: (input: ProjectInput) => request<Project>('/projects', json('POST', input)),
  setProjectVisibility: (id: string, isPublic: boolean) => request<Project>(`/projects/${encodeURIComponent(id)}`, json('PATCH', { is_public: isPublic })),
  devices: () => request<Device[]>('/devices'),
  createDevice: (name: string) => request<DeviceWithToken>('/devices', json('POST', { name })),
  deleteDevice: (id: string) => request<null>(`/devices/${encodeURIComponent(id)}`, json('DELETE')),
  submissions: () => request<Submission[]>('/submissions'),
  createSubmission: (sessionIds: string[], note: string) =>
    request<Submission>('/submissions', json('POST', { session_ids: sessionIds, note })),
  submission: (token: string) => request<Submission>(`/submissions/${encodeURIComponent(token)}`),
};
