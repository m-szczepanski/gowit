import {Commit, DiscardFiles, GetStatus, StageAll, StageFiles, UnstageAll, UnstageFiles} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';

export type StatusResponse = main.StatusResponse;

export class ApiError extends Error {
    code: string;

    constructor(code: string, message: string) {
        super(message);
        this.code = code;
    }
}

/**
 * Typed wrappers over the generated Wails bindings. Every backend call
 * answers with a CallResult envelope where an empty code means success;
 * anything else becomes an ApiError so hooks and components handle exactly
 * one error shape. A transport rejection (bindings not ready) passes
 * through as a plain Error.
 */
function unwrapStatus(res: main.StatusResponse): main.StatusResponse {
    if (res.code) {
        throw new ApiError(res.code, res.message);
    }
    return res;
}

export function getStatus(): Promise<StatusResponse> {
    return GetStatus().then(unwrapStatus);
}

export function stageFiles(paths: string[]): Promise<StatusResponse> {
    return StageFiles(paths).then(unwrapStatus);
}

export function unstageFiles(paths: string[]): Promise<StatusResponse> {
    return UnstageFiles(paths).then(unwrapStatus);
}

export function stageAll(): Promise<StatusResponse> {
    return StageAll().then(unwrapStatus);
}

export function unstageAll(): Promise<StatusResponse> {
    return UnstageAll().then(unwrapStatus);
}

/**
 * Commits the staged index. Unlike the staging calls, no status echo comes
 * back: the status update arrives as a repo:status-changed event instead
 * (the Go side nudges its status worker after a successful commit).
 */
export async function commit(message: string, amend = false): Promise<void> {
    const res = await Commit(message, amend);
    if (res.code) {
        throw new ApiError(res.code, res.message);
    }
}

export function discardFiles(paths: string[]): Promise<StatusResponse> {
    return DiscardFiles(paths).then(unwrapStatus);
}
