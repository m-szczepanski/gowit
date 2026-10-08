import {
    Commit,
    DiscardFiles,
    GetStatus,
    GetSubmodules,
    OpenSubmodule,
    StageAll,
    StageFiles,
    SubmoduleAdd,
    SubmoduleDeinit,
    SubmoduleInitUpdate,
    SubmoduleRemove,
    SubmoduleUpdate,
    UnstageAll,
    UnstageFiles
} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';

export type StatusResponse = main.StatusResponse;
export type SubmodulesResponse = main.SubmodulesResponse;
export type OpenRepositoryResult = main.OpenRepositoryResult;

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
function unwrapCall(res: main.CallResult): void {
    if (res.code) {
        throw new ApiError(res.code, res.message);
    }
}

function unwrapStatus(res: main.StatusResponse): main.StatusResponse {
    unwrapCall(res);
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
export function commit(message: string, amend = false): Promise<void> {
    return Commit(message, amend).then(unwrapCall);
}

export function discardFiles(paths: string[]): Promise<StatusResponse> {
    return DiscardFiles(paths).then(unwrapStatus);
}

function unwrapSubmodules(res: main.SubmodulesResponse): main.SubmodulesResponse {
    unwrapCall(res);
    return res;
}

export function getSubmodules(): Promise<SubmodulesResponse> {
    return GetSubmodules().then(unwrapSubmodules);
}

/**
 * Submodule lifecycle adapters (#76). Like staging, every mutation answers
 * with the fresh StatusResponse, so callers replace the status cache with
 * the echo; the submodules list itself changes with these ops, so hooks
 * additionally invalidate its key.
 */
export function submoduleInitUpdate(recursive: boolean): Promise<StatusResponse> {
    return SubmoduleInitUpdate(recursive).then(unwrapStatus);
}

export function submoduleUpdate(path: string): Promise<StatusResponse> {
    return SubmoduleUpdate(path).then(unwrapStatus);
}

export function submoduleDeinit(path: string): Promise<StatusResponse> {
    return SubmoduleDeinit(path).then(unwrapStatus);
}

export function submoduleRemove(path: string): Promise<StatusResponse> {
    return SubmoduleRemove(path).then(unwrapStatus);
}

export function submoduleAdd(url: string, path: string): Promise<StatusResponse> {
    return SubmoduleAdd(url, path).then(unwrapStatus);
}

export function openSubmodule(path: string): Promise<OpenRepositoryResult> {
    return OpenSubmodule(path).then((res) => {
        unwrapCall(res);
        return res;
    });
}
