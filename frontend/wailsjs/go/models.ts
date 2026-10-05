export namespace config {
	
	export class RecentRepo {
	    path: string;
	    // Go type: time
	    lastOpened: any;
	
	    static createFrom(source: any = {}) {
	        return new RecentRepo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.lastOpened = this.convertValues(source["lastOpened"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	    }
	}

}

export namespace git {
	
	export class BranchStatus {
	    head: string;
	    oid: string;
	    detached: boolean;
	    upstream?: string;
	    ahead: number;
	    behind: number;
	
	    static createFrom(source: any = {}) {
	        return new BranchStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.head = source["head"];
	        this.oid = source["oid"];
	        this.detached = source["detached"];
	        this.upstream = source["upstream"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	    }
	}
	export class MergeStage {
	    stage: number;
	    mode: string;
	    oid: string;
	
	    static createFrom(source: any = {}) {
	        return new MergeStage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stage = source["stage"];
	        this.mode = source["mode"];
	        this.oid = source["oid"];
	    }
	}
	export class FileStatus {
	    xy: string;
	    path: string;
	    origPath?: string;
	    submodule?: string;
	    untracked: boolean;
	    ignored: boolean;
	    conflict: boolean;
	    staged: boolean;
	    unstaged: boolean;
	    change: string;
	    stages?: MergeStage[];
	
	    static createFrom(source: any = {}) {
	        return new FileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.xy = source["xy"];
	        this.path = source["path"];
	        this.origPath = source["origPath"];
	        this.submodule = source["submodule"];
	        this.untracked = source["untracked"];
	        this.ignored = source["ignored"];
	        this.conflict = source["conflict"];
	        this.staged = source["staged"];
	        this.unstaged = source["unstaged"];
	        this.change = source["change"];
	        this.stages = this.convertValues(source["stages"], MergeStage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class CallResult {
	    code: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class FolderDialogResult {
	    code: string;
	    message: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new FolderDialogResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.path = source["path"];
	    }
	}
	export class OpenRepositoryResult {
	    code: string;
	    message: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenRepositoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.path = source["path"];
	    }
	}
	export class StatusResponse {
	    code: string;
	    message: string;
	    branch: git.BranchStatus;
	    files: git.FileStatus[];
	
	    static createFrom(source: any = {}) {
	        return new StatusResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.branch = this.convertValues(source["branch"], git.BranchStatus);
	        this.files = this.convertValues(source["files"], git.FileStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

