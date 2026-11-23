export namespace main {
	
	export class Backup {
	    path: string;
	    time: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Backup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.time = source["time"];
	        this.size = source["size"];
	    }
	}
	export class HostView {
	    alias: string;
	    patterns: string[];
	    wildcard: boolean;
	    section: string;
	    line: number;
	    hostName: string;
	    port: string;
	    user: string;
	    identityFile: string;
	    identitiesOnly: boolean;
	    proxyJump: string;
	    groups: string[];
	    auth: string;
	    plainPassword: boolean;
	    encPassword: boolean;
	    effective: Record<string, string>;
	    options: sshconf.Option[];
	    raw: string;
	
	    static createFrom(source: any = {}) {
	        return new HostView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alias = source["alias"];
	        this.patterns = source["patterns"];
	        this.wildcard = source["wildcard"];
	        this.section = source["section"];
	        this.line = source["line"];
	        this.hostName = source["hostName"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.identityFile = source["identityFile"];
	        this.identitiesOnly = source["identitiesOnly"];
	        this.proxyJump = source["proxyJump"];
	        this.groups = source["groups"];
	        this.auth = source["auth"];
	        this.plainPassword = source["plainPassword"];
	        this.encPassword = source["encPassword"];
	        this.effective = source["effective"];
	        this.options = this.convertValues(source["options"], sshconf.Option);
	        this.raw = source["raw"];
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
	export class KeyInfo {
	    path: string;
	    display: string;
	    public: string;
	    created: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.display = source["display"];
	        this.public = source["public"];
	        this.created = source["created"];
	    }
	}
	export class SaveRequest {
	    origAlias: string;
	    alias: string;
	    section: string;
	    changes: sshconf.Change[];
	
	    static createFrom(source: any = {}) {
	        return new SaveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.origAlias = source["origAlias"];
	        this.alias = source["alias"];
	        this.section = source["section"];
	        this.changes = this.convertValues(source["changes"], sshconf.Change);
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
	export class State {
	    configPath: string;
	    hosts: HostView[];
	    sections: string[];
	    warnings: string[];
	    loadedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.hosts = this.convertValues(source["hosts"], HostView);
	        this.sections = source["sections"];
	        this.warnings = source["warnings"];
	        this.loadedAt = source["loadedAt"];
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

export namespace settings {
	
	export class Settings {
	    terminal: string;
	    configPath: string;
	    keyDir: string;
	    tssh: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.terminal = source["terminal"];
	        this.configPath = source["configPath"];
	        this.keyDir = source["keyDir"];
	        this.tssh = source["tssh"];
	    }
	}

}

export namespace sshconf {
	
	export class Change {
	    key: string;
	    value: string;
	    ext: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Change(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.ext = source["ext"];
	    }
	}
	export class Option {
	    key: string;
	    value: string;
	    ext: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Option(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.ext = source["ext"];
	    }
	}

}

