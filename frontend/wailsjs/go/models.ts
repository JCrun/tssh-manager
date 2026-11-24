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
	export class SyncSave {
	    backend: string;
	    folder: string;
	    webdavURL: string;
	    webdavUser: string;
	    webdavPassword: string;
	    passphrase: string;
	    syncKeys: boolean;
	    auto: boolean;
	    device: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncSave(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.folder = source["folder"];
	        this.webdavURL = source["webdavURL"];
	        this.webdavUser = source["webdavUser"];
	        this.webdavPassword = source["webdavPassword"];
	        this.passphrase = source["passphrase"];
	        this.syncKeys = source["syncKeys"];
	        this.auto = source["auto"];
	        this.device = source["device"];
	    }
	}
	export class SyncView {
	    backend: string;
	    folder: string;
	    webdavURL: string;
	    webdavUser: string;
	    hasWebdavPassword: boolean;
	    hasPassphrase: boolean;
	    syncKeys: boolean;
	    auto: boolean;
	    device: string;
	    keyDir: string;
	    lastSync: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.folder = source["folder"];
	        this.webdavURL = source["webdavURL"];
	        this.webdavUser = source["webdavUser"];
	        this.hasWebdavPassword = source["hasWebdavPassword"];
	        this.hasPassphrase = source["hasPassphrase"];
	        this.syncKeys = source["syncKeys"];
	        this.auto = source["auto"];
	        this.device = source["device"];
	        this.keyDir = source["keyDir"];
	        this.lastSync = source["lastSync"];
	    }
	}
	export class TSSHInfo {
	    path: string;
	    source: string;
	    version: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new TSSHInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.source = source["source"];
	        this.version = source["version"];
	        this.error = source["error"];
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

export namespace syncer {
	
	export class Changes {
	    fromRemote: string[];
	    fromLocal: string[];
	
	    static createFrom(source: any = {}) {
	        return new Changes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fromRemote = source["fromRemote"];
	        this.fromLocal = source["fromLocal"];
	    }
	}
	export class Conflict {
	    id: string;
	    kind: string;
	    name: string;
	    local: string;
	    remote: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new Conflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.local = source["local"];
	        this.remote = source["remote"];
	        this.note = source["note"];
	    }
	}
	export class Result {
	    status: string;
	    remote: string;
	    remoteDevice: string;
	    changes: Changes;
	    conflicts: Conflict[];
	    warnings: string[];
	    backup: string;
	    time: number;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.remote = source["remote"];
	        this.remoteDevice = source["remoteDevice"];
	        this.changes = this.convertValues(source["changes"], Changes);
	        this.conflicts = this.convertValues(source["conflicts"], Conflict);
	        this.warnings = source["warnings"];
	        this.backup = source["backup"];
	        this.time = source["time"];
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

