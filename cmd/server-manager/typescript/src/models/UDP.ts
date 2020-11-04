// this file was automatically generated, DO NOT EDIT

// helpers
const maxUnixTSInSeconds = 9999999999;

function ParseDate(d: Date | number | string): Date {
	if (d instanceof Date) return d;
	if (typeof d === 'number') {
		if (d > maxUnixTSInSeconds) return new Date(d);
		return new Date(d * 1000); // go ts
	}
	return new Date(d);
}

function ParseNumber(v: number | string, isInt = false): number {
	if (!v) return 0;
	if (typeof v === 'number') return v;
	return (isInt ? parseInt(v) : parseFloat(v)) || 0;
}

function FromArray<T>(Ctor: { new(v: any): T }, data?: any[] | any, def = null): T[] | null {
	if (!data || !Object.keys(data).length) return def;
	const d = Array.isArray(data) ? data : [data];
	return d.map((v: any) => new Ctor(v));
}

function ToObject(o: any, typeOrCfg: any = {}, child = false): any {
	if (!o) return null;
	if (typeof o.toObject === 'function' && child) return o.toObject();

	switch (typeof o) {
		case 'string':
			return typeOrCfg === 'number' ? ParseNumber(o) : o;
		case 'boolean':
		case 'number':
			return o;
	}

	if (o instanceof Date) {
		return typeOrCfg === 'string' ? o.toISOString() : Math.floor(o.getTime() / 1000);
	}

	if (Array.isArray(o)) return o.map((v: any) => ToObject(v, typeOrCfg, true));

	const d: any = {};

	for (const k of Object.keys(o)) {
		const v: any = o[k];
		if (!v) continue;
		d[k] = ToObject(v, typeOrCfg[k] || {}, true);
	}

	return d;
}

// structs
// struct2ts:justapengu.in/acsm/pkg/udp.SessionInfo
class SessionInfo {
    Version: number;
    SessionIndex: number;
    CurrentSessionIndex: number;
    SessionCount: number;
    ServerName: string;
    Track: string;
    TrackConfig: string;
    Name: string;
    Type: number;
    Time: number;
    Laps: number;
    WaitTime: number;
    AmbientTemp: number;
    RoadTemp: number;
    WeatherGraphics: string;
    ElapsedMilliseconds: number;
    IsSolo: boolean;
    EventType: number;

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.Version = ('Version' in d) ? d.Version as number : 0;
        this.SessionIndex = ('SessionIndex' in d) ? d.SessionIndex as number : 0;
        this.CurrentSessionIndex = ('CurrentSessionIndex' in d) ? d.CurrentSessionIndex as number : 0;
        this.SessionCount = ('SessionCount' in d) ? d.SessionCount as number : 0;
        this.ServerName = ('ServerName' in d) ? d.ServerName as string : '';
        this.Track = ('Track' in d) ? d.Track as string : '';
        this.TrackConfig = ('TrackConfig' in d) ? d.TrackConfig as string : '';
        this.Name = ('Name' in d) ? d.Name as string : '';
        this.Type = ('Type' in d) ? d.Type as number : 0;
        this.Time = ('Time' in d) ? d.Time as number : 0;
        this.Laps = ('Laps' in d) ? d.Laps as number : 0;
        this.WaitTime = ('WaitTime' in d) ? d.WaitTime as number : 0;
        this.AmbientTemp = ('AmbientTemp' in d) ? d.AmbientTemp as number : 0;
        this.RoadTemp = ('RoadTemp' in d) ? d.RoadTemp as number : 0;
        this.WeatherGraphics = ('WeatherGraphics' in d) ? d.WeatherGraphics as string : '';
        this.ElapsedMilliseconds = ('ElapsedMilliseconds' in d) ? d.ElapsedMilliseconds as number : 0;
        this.IsSolo = ('IsSolo' in d) ? d.IsSolo as boolean : false;
        this.EventType = ('EventType' in d) ? d.EventType as number : 0;
    }

    toObject(): any {
        const cfg: any = {};
        cfg.Version = 'number';
        cfg.SessionIndex = 'number';
        cfg.CurrentSessionIndex = 'number';
        cfg.SessionCount = 'number';
        cfg.Type = 'number';
        cfg.Time = 'number';
        cfg.Laps = 'number';
        cfg.WaitTime = 'number';
        cfg.AmbientTemp = 'number';
        cfg.RoadTemp = 'number';
        cfg.ElapsedMilliseconds = 'number';
        cfg.EventType = 'number';
        return ToObject(this, cfg);
    }
}

// struct2ts:justapengu.in/acsm/internal/acserver.Vector3F
class Vector3F {
    X: number;
    Y: number;
    Z: number;

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.X = ('X' in d) ? d.X as number : 0;
        this.Y = ('Y' in d) ? d.Y as number : 0;
        this.Z = ('Z' in d) ? d.Z as number : 0;
    }

    toObject(): any {
        const cfg: any = {};
        cfg.X = 'number';
        cfg.Y = 'number';
        cfg.Z = 'number';
        return ToObject(this, cfg);
    }
}

// struct2ts:justapengu.in/acsm/pkg/udp.CarUpdate
class CarUpdate {
    CarID: number;
    Pos: Vector3F;
    Velocity: Vector3F;
    Gear: number;
    EngineRPM: number;
    NormalisedSplinePos: number;
    SteerAngle: number;
    StatusBytes: number;

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.CarID = ('CarID' in d) ? d.CarID as number : 0;
        this.Pos = new Vector3F(d.Pos);
        this.Velocity = new Vector3F(d.Velocity);
        this.Gear = ('Gear' in d) ? d.Gear as number : 0;
        this.EngineRPM = ('EngineRPM' in d) ? d.EngineRPM as number : 0;
        this.NormalisedSplinePos = ('NormalisedSplinePos' in d) ? d.NormalisedSplinePos as number : 0;
        this.SteerAngle = ('SteerAngle' in d) ? d.SteerAngle as number : 0;
        this.StatusBytes = ('StatusBytes' in d) ? d.StatusBytes as number : 0;
    }

    toObject(): any {
        const cfg: any = {};
        cfg.CarID = 'number';
        cfg.Gear = 'number';
        cfg.EngineRPM = 'number';
        cfg.NormalisedSplinePos = 'number';
        cfg.SteerAngle = 'number';
        cfg.StatusBytes = 'number';
        return ToObject(this, cfg);
    }
}

// struct2ts:justapengu.in/acsm/pkg/udp.LapCompletedCar
class LapCompletedCar {
    CarID: number;
    LapTime: number;
    Laps: number;
    Completed: number;

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.CarID = ('CarID' in d) ? d.CarID as number : 0;
        this.LapTime = ('LapTime' in d) ? d.LapTime as number : 0;
        this.Laps = ('Laps' in d) ? d.Laps as number : 0;
        this.Completed = ('Completed' in d) ? d.Completed as number : 0;
    }

    toObject(): any {
        const cfg: any = {};
        cfg.CarID = 'number';
        cfg.LapTime = 'number';
        cfg.Laps = 'number';
        cfg.Completed = 'number';
        return ToObject(this, cfg);
    }
}

// struct2ts:justapengu.in/acsm/pkg/udp.LapCompleted
class LapCompleted {
    CarID: number;
    LapTime: number;
    Cuts: number;
    CarsCount: number;
    Tyres: string;
    Cars: LapCompletedCar[] | null;

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.CarID = ('CarID' in d) ? d.CarID as number : 0;
        this.LapTime = ('LapTime' in d) ? d.LapTime as number : 0;
        this.Cuts = ('Cuts' in d) ? d.Cuts as number : 0;
        this.CarsCount = ('CarsCount' in d) ? d.CarsCount as number : 0;
        this.Tyres = ('Tyres' in d) ? d.Tyres as string : '';
        this.Cars = Array.isArray(d.Cars) ? d.Cars.map((v: any) => new LapCompletedCar(v)) : null;
    }

    toObject(): any {
        const cfg: any = {};
        cfg.CarID = 'number';
        cfg.LapTime = 'number';
        cfg.Cuts = 'number';
        cfg.CarsCount = 'number';
        return ToObject(this, cfg);
    }
}

// struct2ts:justapengu.in/acsm/pkg/udp.CollisionWithEnvironment
class CollisionWithEnvironment {
    CarID: number;
    ImpactSpeed: number;
    WorldPos: Vector3F;
    RelPos: Vector3F;
    DamageZones: number[];

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.CarID = ('CarID' in d) ? d.CarID as number : 0;
        this.ImpactSpeed = ('ImpactSpeed' in d) ? d.ImpactSpeed as number : 0;
        this.WorldPos = new Vector3F(d.WorldPos);
        this.RelPos = new Vector3F(d.RelPos);
        this.DamageZones = ('DamageZones' in d) ? d.DamageZones as number[] : [];
    }

    toObject(): any {
        const cfg: any = {};
        cfg.CarID = 'number';
        cfg.ImpactSpeed = 'number';
        return ToObject(this, cfg);
    }
}

// struct2ts:justapengu.in/acsm/pkg/udp.CollisionWithCar
class CollisionWithCar {
    CarID: number;
    OtherCarID: number;
    ImpactSpeed: number;
    WorldPos: Vector3F;
    RelPos: Vector3F;
    DamageZones: number[];
    OtherDamageZones: number[];

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.CarID = ('CarID' in d) ? d.CarID as number : 0;
        this.OtherCarID = ('OtherCarID' in d) ? d.OtherCarID as number : 0;
        this.ImpactSpeed = ('ImpactSpeed' in d) ? d.ImpactSpeed as number : 0;
        this.WorldPos = new Vector3F(d.WorldPos);
        this.RelPos = new Vector3F(d.RelPos);
        this.DamageZones = ('DamageZones' in d) ? d.DamageZones as number[] : [];
        this.OtherDamageZones = ('OtherDamageZones' in d) ? d.OtherDamageZones as number[] : [];
    }

    toObject(): any {
        const cfg: any = {};
        cfg.CarID = 'number';
        cfg.OtherCarID = 'number';
        cfg.ImpactSpeed = 'number';
        return ToObject(this, cfg);
    }
}

// struct2ts:justapengu.in/acsm/pkg/udp.Chat
class Chat {
    CarID: number;
    RecipientCarID: number;
    Message: string;
    DriverGUID: string;
    DriverName: string;
    Time: Date;

    constructor(data?: any) {
        const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
        this.CarID = ('CarID' in d) ? d.CarID as number : 0;
        this.RecipientCarID = ('RecipientCarID' in d) ? d.RecipientCarID as number : 0;
        this.Message = ('Message' in d) ? d.Message as string : '';
        this.DriverGUID = ('DriverGUID' in d) ? d.DriverGUID as string : '';
        this.DriverName = ('DriverName' in d) ? d.DriverName as string : '';
        this.Time = ('Time' in d) ? ParseDate(d.Time) : new Date();
    }

    toObject(): any {
        const cfg: any = {};
        cfg.CarID = 'number';
        cfg.RecipientCarID = 'number';
        cfg.Time = 'string';
        return ToObject(this, cfg);
    }
}

// exports
export {
    SessionInfo,
    Vector3F,
    CarUpdate,
    LapCompletedCar,
    LapCompleted,
    CollisionWithEnvironment,
    CollisionWithCar,
    Chat,
    ParseDate,
    ParseNumber,
    FromArray,
    ToObject,
};
