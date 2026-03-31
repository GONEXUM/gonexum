export namespace main {
	
	export class MediaInfo {
	    resolution: string;
	    videoCodec: string;
	    audioCodec: string;
	    audioLanguages: string;
	    hdrFormat: string;
	    source: string;
	    duration: string;
	    fileSize: number;
	    width: number;
	    height: number;
	    bitrate: number;
	    frameRate: number;
	
	    static createFrom(source: any = {}) {
	        return new MediaInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resolution = source["resolution"];
	        this.videoCodec = source["videoCodec"];
	        this.audioCodec = source["audioCodec"];
	        this.audioLanguages = source["audioLanguages"];
	        this.hdrFormat = source["hdrFormat"];
	        this.source = source["source"];
	        this.duration = source["duration"];
	        this.fileSize = source["fileSize"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.bitrate = source["bitrate"];
	        this.frameRate = source["frameRate"];
	    }
	}
	export class Settings {
	    apiKey: string;
	    passkey: string;
	    tmdbApiKey: string;
	    trackerUrl: string;
	    outputDir: string;
	    nfoTemplate: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.passkey = source["passkey"];
	        this.tmdbApiKey = source["tmdbApiKey"];
	        this.trackerUrl = source["trackerUrl"];
	        this.outputDir = source["outputDir"];
	        this.nfoTemplate = source["nfoTemplate"];
	    }
	}
	export class TMDBDetails {
	    id: number;
	    title: string;
	    year: string;
	    overview: string;
	    posterPath: string;
	    mediaType: string;
	    genres: string[];
	    director: string;
	    rating: number;
	    runtime: number;
	
	    static createFrom(source: any = {}) {
	        return new TMDBDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.year = source["year"];
	        this.overview = source["overview"];
	        this.posterPath = source["posterPath"];
	        this.mediaType = source["mediaType"];
	        this.genres = source["genres"];
	        this.director = source["director"];
	        this.rating = source["rating"];
	        this.runtime = source["runtime"];
	    }
	}
	export class TMDBResult {
	    id: number;
	    title: string;
	    year: string;
	    posterPath: string;
	    mediaType: string;
	    overview: string;
	    popularity: number;
	
	    static createFrom(source: any = {}) {
	        return new TMDBResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.year = source["year"];
	        this.posterPath = source["posterPath"];
	        this.mediaType = source["mediaType"];
	        this.overview = source["overview"];
	        this.popularity = source["popularity"];
	    }
	}
	export class TorrentResult {
	    filePath: string;
	    infoHash: string;
	    name: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new TorrentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.infoHash = source["infoHash"];
	        this.name = source["name"];
	        this.size = source["size"];
	    }
	}
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    available: boolean;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.url = source["url"];
	    }
	}
	export class UploadMedia {
	    resolution: string;
	    video_codec: string;
	    audio_codec: string;
	    audio_languages: string;
	    hdr_format?: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadMedia(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resolution = source["resolution"];
	        this.video_codec = source["video_codec"];
	        this.audio_codec = source["audio_codec"];
	        this.audio_languages = source["audio_languages"];
	        this.hdr_format = source["hdr_format"];
	        this.source = source["source"];
	    }
	}
	export class UploadParams {
	    torrentPath: string;
	    nfoContent: string;
	    name: string;
	    categoryId: number;
	    description: string;
	    tmdbId: number;
	    tmdbType: string;
	    resolution: string;
	    videoCodec: string;
	    audioCodec: string;
	    audioLanguages: string;
	    hdrFormat: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.torrentPath = source["torrentPath"];
	        this.nfoContent = source["nfoContent"];
	        this.name = source["name"];
	        this.categoryId = source["categoryId"];
	        this.description = source["description"];
	        this.tmdbId = source["tmdbId"];
	        this.tmdbType = source["tmdbType"];
	        this.resolution = source["resolution"];
	        this.videoCodec = source["videoCodec"];
	        this.audioCodec = source["audioCodec"];
	        this.audioLanguages = source["audioLanguages"];
	        this.hdrFormat = source["hdrFormat"];
	        this.source = source["source"];
	    }
	}
	export class UploadResponse {
	    success: boolean;
	    torrent_id: number;
	    info_hash: string;
	    name: string;
	    size: number;
	    status: string;
	    url: string;
	    media: UploadMedia;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.torrent_id = source["torrent_id"];
	        this.info_hash = source["info_hash"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.status = source["status"];
	        this.url = source["url"];
	        this.media = this.convertValues(source["media"], UploadMedia);
	        this.error = source["error"];
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

