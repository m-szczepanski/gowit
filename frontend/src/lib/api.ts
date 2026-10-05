import {GetStatus, StageAll, StageFiles, UnstageAll, UnstageFiles} from '../../wailsjs/go/main/App';
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
 * anything else (or a transport rejection) becomes an ApiError so hooks
 * and components handle exactly one error shape. Backend structs arrive
 * as plain objects over JSON, so results are rebuilt through the generated
 * createFrom factories.
 */
function toStatusResponse(res: StatusResponse): StatusResponse {
    if (res.code) {
        throw new ApiError(res.code, res.message);
    }
    return main.StatusResponse.createFrom(res);
}

export function getStatus(): Promise<StatusResponse> {
    return GetStatus().then(toStatusResponse);
}

export function stageFiles(paths: string[]): Promise<StatusResponse> {
    return StageFiles(paths).then(toStatusResponse);
}

export function unstageFiles(paths: string[]): Promise<StatusResponse> {
    return UnstageFiles(paths).then(toStatusResponse);
}

export function stageAll(): Promise<StatusResponse> {
    return StageAll().then(toStatusResponse);
}

export function unstageAll(): Promise<StatusResponse> {
    return UnstageAll().then(toStatusResponse);
}
