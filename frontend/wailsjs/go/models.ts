export namespace analyzer {
	
	export class CalcStep {
	    desc: string;
	    expr: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new CalcStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.desc = source["desc"];
	        this.expr = source["expr"];
	        this.value = source["value"];
	    }
	}
	export class InputValue {
	    source: string;
	    item: string;
	    year: string;
	    value: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new InputValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.item = source["item"];
	        this.year = source["year"];
	        this.value = source["value"];
	        this.note = source["note"];
	    }
	}
	export class CalcTrace {
	    indicator: string;
	    year: string;
	    formula: string;
	    inputs: Record<string, InputValue>;
	    steps: CalcStep[];
	    result: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new CalcTrace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.indicator = source["indicator"];
	        this.year = source["year"];
	        this.formula = source["formula"];
	        this.inputs = this.convertValues(source["inputs"], InputValue, true);
	        this.steps = this.convertValues(source["steps"], CalcStep);
	        this.result = source["result"];
	        this.note = source["note"];
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
	export class StepResult {
	    stepNum: number;
	    stepName: string;
	    yearlyData: Record<string, any>;
	    conclusion: string;
	    pass: Record<string, boolean>;
	    traces: CalcTrace[];
	
	    static createFrom(source: any = {}) {
	        return new StepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stepNum = source["stepNum"];
	        this.stepName = source["stepName"];
	        this.yearlyData = source["yearlyData"];
	        this.conclusion = source["conclusion"];
	        this.pass = source["pass"];
	        this.traces = this.convertValues(source["traces"], CalcTrace);
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
	export class AnalysisReport {
	    symbol: string;
	    companyName: string;
	    years: string[];
	    stepResults: StepResult[];
	    passSummary: Record<string, Array<PassItem>>;
	    score: Record<string, number>;
	    overallGrade: string;
	    markdownContent: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.companyName = source["companyName"];
	        this.years = source["years"];
	        this.stepResults = this.convertValues(source["stepResults"], StepResult);
	        this.passSummary = this.convertValues(source["passSummary"], Array<PassItem>, true);
	        this.score = source["score"];
	        this.overallGrade = source["overallGrade"];
	        this.markdownContent = source["markdownContent"];
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
	
	
	
	export class PassItem {
	    year: string;
	    passed: boolean;
	    value: any;
	
	    static createFrom(source: any = {}) {
	        return new PassItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.year = source["year"];
	        this.passed = source["passed"];
	        this.value = source["value"];
	    }
	}

}

export namespace downloader {
	
	export class KlineData {
	    time: string;
	    open: number;
	    close: number;
	    low: number;
	    high: number;
	    volume: number;
	    amount: number;
	
	    static createFrom(source: any = {}) {
	        return new KlineData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.open = source["open"];
	        this.close = source["close"];
	        this.low = source["low"];
	        this.high = source["high"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	    }
	}
	export class StockConcepts {
	    concepts: string[];
	    wind: string;
	
	    static createFrom(source: any = {}) {
	        return new StockConcepts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.concepts = source["concepts"];
	        this.wind = source["wind"];
	    }
	}
	export class StockQuote {
	    currentPrice: number;
	    changePercent: number;
	    changeAmount: number;
	    volume: number;
	    turnoverAmount: number;
	    turnoverRate: number;
	    amplitude: number;
	    high: number;
	    low: number;
	    open: number;
	    previousClose: number;
	    circulatingMarketCap: number;
	    volumeRatio: number;
	    pe: number;
	    pb: number;
	    marketCap: number;
	    quoteTime: string;
	
	    static createFrom(source: any = {}) {
	        return new StockQuote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentPrice = source["currentPrice"];
	        this.changePercent = source["changePercent"];
	        this.changeAmount = source["changeAmount"];
	        this.volume = source["volume"];
	        this.turnoverAmount = source["turnoverAmount"];
	        this.turnoverRate = source["turnoverRate"];
	        this.amplitude = source["amplitude"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.open = source["open"];
	        this.previousClose = source["previousClose"];
	        this.circulatingMarketCap = source["circulatingMarketCap"];
	        this.volumeRatio = source["volumeRatio"];
	        this.pe = source["pe"];
	        this.pb = source["pb"];
	        this.marketCap = source["marketCap"];
	        this.quoteTime = source["quoteTime"];
	    }
	}
	export class ValidationResult {
	    year: string;
	    indicator: string;
	    hsf10Value: number;
	    dcValue: number;
	    diffPercent: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.year = source["year"];
	        this.indicator = source["indicator"];
	        this.hsf10Value = source["hsf10Value"];
	        this.dcValue = source["dcValue"];
	        this.diffPercent = source["diffPercent"];
	        this.status = source["status"];
	    }
	}

}

export namespace main {
	
	export class CacheStatus {
	    unchanged: boolean;
	    lastAnalysisAt: string;
	    dataChanged: boolean;
	    comparablesChanged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CacheStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unchanged = source["unchanged"];
	        this.lastAnalysisAt = source["lastAnalysisAt"];
	        this.dataChanged = source["dataChanged"];
	        this.comparablesChanged = source["comparablesChanged"];
	    }
	}
	export class DownloadResult {
	    success: boolean;
	    message: string;
	    years: string[];
	    validation: downloader.ValidationResult[];
	
	    static createFrom(source: any = {}) {
	        return new DownloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.years = source["years"];
	        this.validation = this.convertValues(source["validation"], downloader.ValidationResult);
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
	export class HistoryMeta {
	    timestamp: string;
	    source: string;
	    sourceName: string;
	    years: string[];
	
	    static createFrom(source: any = {}) {
	        return new HistoryMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.source = source["source"];
	        this.sourceName = source["sourceName"];
	        this.years = source["years"];
	    }
	}
	export class ImportResult {
	    success: boolean;
	    message: string;
	    balanceSheet: string[];
	    income: string[];
	    cashFlow: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.balanceSheet = source["balanceSheet"];
	        this.income = source["income"];
	        this.cashFlow = source["cashFlow"];
	    }
	}
	export class StockInfo {
	    code: string;
	    name: string;
	    market: string;
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	    }
	}
	export class StockProfile {
	    industry: string;
	    listingDate: string;
	    totalShares: number;
	    marketCap: number;
	    pe: number;
	    pb: number;
	    eps: number;
	    chairman: string;
	    controller: string;
	    chairmanGender: string;
	    chairmanAge: string;
	    chairmanNationality: string;
	    chairmanHoldRatio: string;
	    politicalAffiliation: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StockProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.industry = source["industry"];
	        this.listingDate = source["listingDate"];
	        this.totalShares = source["totalShares"];
	        this.marketCap = source["marketCap"];
	        this.pe = source["pe"];
	        this.pb = source["pb"];
	        this.eps = source["eps"];
	        this.chairman = source["chairman"];
	        this.controller = source["controller"];
	        this.chairmanGender = source["chairmanGender"];
	        this.chairmanAge = source["chairmanAge"];
	        this.chairmanNationality = source["chairmanNationality"];
	        this.chairmanHoldRatio = source["chairmanHoldRatio"];
	        this.politicalAffiliation = source["politicalAffiliation"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class WatchlistActivitySummary {
	    code: string;
	    score: number;
	    stars: number;
	    grade: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistActivitySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.score = source["score"];
	        this.stars = source["stars"];
	        this.grade = source["grade"];
	    }
	}
	export class WatchlistItem {
	    code: string;
	    name: string;
	    market: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	    }
	}

}

