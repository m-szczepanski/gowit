export namespace main {
	
	export class FolderDialogResult {
	    path: string;
	    code: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new FolderDialogResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}

}

