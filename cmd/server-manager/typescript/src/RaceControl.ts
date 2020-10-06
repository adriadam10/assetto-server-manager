import {
    RaceControl as RaceControlData,
    RaceControlDriverMapRaceControlDriver as Driver,
    RaceControlDriverMapRaceControlDriverSessionCarInfo as SessionCarInfo
} from "./models/RaceControl";

import {CarUpdate, CarUpdateVector3F} from "./models/UDP";
import {randomColor} from "randomcolor/randomColor";
import {msToTime, prettifyName} from "./utils";
import moment from "moment";
import ReconnectingWebSocket from "reconnecting-websocket";
import ClickEvent = JQuery.ClickEvent;
import ChangeEvent = JQuery.ChangeEvent;

interface WSMessage {
    Message: any;
    EventType: number;
}

const EventCollisionWithCar = 10,
    EventCollisionWithEnv = 11,
    EventNewSession = 50,
    EventNewConnection = 51,
    EventConnectionClosed = 52,
    EventCarUpdate = 53,
    EventCarInfo = 54,
    EventEndSession = 55,
    EventVersion = 56,
    EventChat = 57,
    EventClientLoaded = 58,
    EventSessionInfo = 59,
    EventError = 60,
    EventLapCompleted = 73,
    EventClientEvent = 130,
    EventRaceControl = 200,
    EventTyresChanged = 101
;

const GolangZeroTime = "0001-01-01T00:00:00Z";

interface SimpleCollision {
    WorldPos: CarUpdateVector3F
}

interface WebsocketHandler {
    handleWebsocketMessage(message: WSMessage): void;

    onTrackChange(track: string, trackLayout: string): void;
}

export class RaceControl {
    private readonly liveMap: LiveMap = new LiveMap(this);
    private readonly liveTimings: LiveTimings = new LiveTimings(this, this.liveMap);
    private readonly $eventTitle: JQuery<HTMLHeadElement>;
    public status: RaceControlData;
    private firstLoad: boolean = true;

    private track: string = "";
    private trackLayout: string = "";
    private weatherGraphics: string = "";

    constructor() {
        this.$eventTitle = $("#event-title");
        this.status = new RaceControlData();

        if (!this.$eventTitle.length) {
            return;
        }

        let ws = new ReconnectingWebSocket(((window.location.protocol === "https:") ? "wss://" : "ws://") + window.location.host + "/api/race-control", [], {
            minReconnectionDelay: 0,
        });

        ws.onmessage = this.handleWebsocketMessage.bind(this);

        $(window).on('beforeunload', () => {
            ws.close();
        });

        this.handleIFrames();
        setInterval(this.showEventCompletion.bind(this), 1000);
        this.$eventTitle.on("click", function (e: ClickEvent) {
            e.preventDefault();
        });
    }

    private handleWebsocketMessage(ev: MessageEvent): void {
        let message = JSON.parse(ev.data) as WSMessage;

        if (!message) {
            return;
        }

        switch (message.EventType) {
            case EventVersion:
                location.reload();
                return;
            case EventRaceControl:
                this.status = new RaceControlData(message.Message);

                if (this.status.SessionInfo.Track !== this.track || this.status.SessionInfo.TrackConfig !== this.trackLayout) {
                    this.track = this.status.SessionInfo.Track;
                    this.trackLayout = this.status.SessionInfo.TrackConfig;
                    this.liveMap.onTrackChange(this.track, this.trackLayout);
                    this.liveTimings.onTrackChange(this.track, this.trackLayout);
                    this.onTrackChange(this.track, this.trackLayout);
                }

                if (this.status.SessionInfo.WeatherGraphics !== this.weatherGraphics) {
                    this.weatherGraphics = this.status.SessionInfo.WeatherGraphics;
                    this.showTrackWeatherImage();
                }

                this.$eventTitle.text((this.status.SessionInfo.IsSolo ? "Solo " : "") + RaceControl.getSessionType(this.status.SessionInfo.Type) + " at " + this.status.TrackInfo!.name);
                $("#track-location").text(this.status.TrackInfo.city + ", " + this.status.TrackInfo.country);

                this.buildSessionInfo();

                if (this.firstLoad) {
                    this.showTrackWeatherImage();
                }

                this.firstLoad = false;
                break;
            case EventNewSession:
            case EventSessionInfo:
                this.showTrackWeatherImage();
                break;
            case EventChat:
                let $chatContainer = $("#chat-container");

                let chatMessage = $(".chat-message-template").first().clone();
                let chatMessageSender = $("<span>");

                let dt = new Date(message.Message.Time);

                let minutes = dt.getMinutes();
                let minutesString = "";

                let hours = dt.getHours();
                let hoursString = "";

                if (minutes < 10) {
                    minutesString = "0"+minutes;
                } else {
                    minutesString = minutes.toLocaleString();
                }

                if (hours < 10) {
                    hoursString = "0"+hours;
                } else {
                    hoursString = hours.toLocaleString();
                }

                chatMessageSender.attr(
                    "style", "color: " + randomColorForDriver(message.Message.DriverGUID)
                ).text(
                    hoursString + ":" + minutesString + " " + message.Message.DriverName + ": "
                );

                chatMessage.text(message.Message.Message);
                chatMessage.addClass("chat-message");
                chatMessageSender.addClass("chat-message-sender");

                $chatContainer.append(chatMessageSender);
                $chatContainer.append(chatMessage);

                if ($chatContainer.find(".chat-message").length > 50) {
                    $chatContainer.find(".chat-message").first().remove();
                    $chatContainer.find(".chat-message-sender").first().remove();
                }

                $chatContainer.scrollTop($chatContainer.prop('scrollHeight'));

                break
        }

        this.liveMap.handleWebsocketMessage(message);
        this.liveTimings.handleWebsocketMessage(message);
    }

    private static getSessionType(sessionIndex: number): string {
        switch (sessionIndex) {
            case 0:
                return "Booking";
            case 1:
                return "Practice";
            case 2:
                return "Qualifying";
            case 3:
                return "Race";
            default:
                return "Unknown session";
        }
    }

    public refreshTimings(): void {
        this.liveTimings.refresh();
    }

    private showEventCompletion() {
        let timeRemaining = "";

        let elapsedTime = moment.duration(moment().utc().diff(moment(this.status.SessionStartTime).utc())).asMilliseconds();
        let isInWaitTime = elapsedTime < (this.status.SessionInfo.WaitTime * 1000);

        if (isInWaitTime) {
            timeRemaining = "Countdown: " + msToTime((this.status.SessionInfo.WaitTime * 1000) - elapsedTime, false, 1);
        } else {
            // Get lap/laps or time/totalTime
            if (this.status.SessionInfo.Time > 0) {
                let timeInMS = (this.status.SessionInfo.Time * 60 * 1000) + (this.status.SessionInfo.WaitTime * 1000) - moment.duration(moment().utc().diff(moment(this.status.SessionStartTime).utc())).asMilliseconds();

                let days = Math.floor(timeInMS/8.64e+7);

                timeRemaining = msToTime(timeInMS, false, 0);

                if (days > 0) {
                    let dayText = " day + ";

                    if ( days > 1) {
                        dayText = " days + ";
                    }

                    timeRemaining = days + dayText + timeRemaining;
                }
            } else if (this.status.SessionInfo.Laps > 0) {
                let lapsCompleted = 0;

                if (this.status.ConnectedDrivers && this.status.ConnectedDrivers.GUIDsInPositionalOrder.length > 0) {
                    let driver = this.status.ConnectedDrivers.Drivers[this.status.ConnectedDrivers.GUIDsInPositionalOrder[0]];

                    if (driver.TotalNumLaps > 0) {
                        lapsCompleted = driver.TotalNumLaps;
                    }
                }

                timeRemaining = this.status.SessionInfo.Laps - lapsCompleted + " laps remaining";
            }
        }

        let $raceTime = $("#race-time");
        $raceTime.text(timeRemaining);
    }

    public onTrackChange(track: string, layout: string): void {
        $("#trackImage").attr("src", this.getTrackImageURL());

        $("#track-description").text(this.status.TrackInfo.description);
        $("#track-length").text(this.status.TrackInfo["length"]);
        $("#track-pitboxes").text(this.status.TrackInfo.pitboxes);
        $("#track-width").text(this.status.TrackInfo.width);
        $("#track-run").text(this.status.TrackInfo.run);
    }

    private buildSessionInfo() {
        let $roadTempWrapper = $("#road-temp-wrapper");
        $roadTempWrapper.attr("style", "background-color: " + getColorForPercentage(this.status.SessionInfo.RoadTemp / 40));
        $roadTempWrapper.attr("data-original-title", "Road Temp: " + this.status.SessionInfo.RoadTemp + "°C");

        let $roadTempText = $("#road-temp-text");
        $roadTempText.text(this.status.SessionInfo.RoadTemp + "°C");

        let $ambientTempWrapper = $("#ambient-temp-wrapper");
        $ambientTempWrapper.attr("style", "background-color: " + getColorForPercentage(this.status.SessionInfo.AmbientTemp / 40));
        $ambientTempWrapper.attr("data-original-title", "Ambient Temp: " + this.status.SessionInfo.AmbientTemp + "°C");

        let $ambientTempText = $("#ambient-temp-text");
        $ambientTempText.text(this.status.SessionInfo.AmbientTemp + "°C");

        $("#event-name").text(this.status.SessionInfo.Name);
        $("#event-type").text(RaceControl.getSessionType(this.status.SessionInfo.Type));
    }

    private showTrackWeatherImage(): void {
        let $currentWeather = $("#weatherImage");

        // Fix for sol weathers with time info in this format:
        // sol_05_Broken%20Clouds_type=18_time=0_mult=20_start=1551792960/preview.jpg
        let pathCorrected = this.status.SessionInfo.WeatherGraphics.split("_");

        for (let i = 0; i < pathCorrected.length; i++) {
            if (pathCorrected[i].indexOf("type=") !== -1) {
                pathCorrected.splice(i);
                break;
            }
        }

        let pathFinal = pathCorrected.join("_");

        $.get("/content/weather/" + pathFinal + "/preview.jpg").done(function () {
            // preview for skin exists
            $currentWeather.attr("src", "/content/weather/" + pathFinal + "/preview.jpg").show();
        }).fail(function () {
            // preview doesn't exist, load default fall back image
            $currentWeather.hide();
        });

        const weatherName = "Current Weather: " + prettifyName(pathFinal, false);

        $currentWeather.attr("data-original-title", weatherName);
        $currentWeather.attr("alt", weatherName);
    }

    private getTrackImageURL(): string {
        if (!this.status) {
            return "";
        }

        const sessionInfo = this.status.SessionInfo;

        return "/content/tracks/" + sessionInfo.Track + "/ui" + (!!sessionInfo.TrackConfig ? "/" + sessionInfo.TrackConfig : "") + "/preview.png";
    }

    private handleIFrames(): void {
        const $document = $(document);

        $document.on("change", ".live-frame-link", function (e: ChangeEvent) {
            let $this = $(e.currentTarget) as JQuery<HTMLInputElement>;
            let value = $this.val() as string;

            if (value) {
                let $liveTimingFrame = $this.closest(".live-frame-wrapper").find(".live-frame");
                $this.closest(".live-frame-wrapper").find(".embed-responsive").attr("class", "embed-responsive embed-responsive-16by9");

                // if somebody pasted an embed code just grab the actual link
                if (value.startsWith('<iframe')) {
                    let res = value.split('"');

                    for (let i = 0; i < res.length; i++) {
                        if (res[i] === " src=") {
                            if (res[i + 1]) {
                                $liveTimingFrame.attr("src", res[i + 1]);
                            }

                            $this.val(res[i + 1]);
                        }
                    }
                } else {
                    $liveTimingFrame.attr("src", value);
                }
            }
        });

        $document.on("click", ".remove-live-frame", function (e: ClickEvent) {
            $(e.currentTarget).closest(".live-frame-wrapper").remove();
        });

        $document.find("#add-live-frame").click(function () {
            let $copy = $document.find(".live-frame-wrapper").first().clone();

            $copy.removeClass("d-none");
            $copy.find(".embed-responsive").attr("class", "d-none embed-responsive embed-responsive-16by9");
            $copy.find(".frame-input").removeClass("ml-0");

            $document.find(".live-frame-wrapper").last().after($copy);
        });
    }
}

declare var useMPH: boolean;

class LiveMap implements WebsocketHandler {
    private mapImageHasLoaded: boolean = false;

    private readonly $map: JQuery<HTMLDivElement>;
    private readonly $trackMapImage: JQuery<HTMLImageElement>;
    private readonly raceControl: RaceControl;

    constructor(raceControl: RaceControl) {
        this.$map = $("#map");
        this.raceControl = raceControl;
        this.$trackMapImage = this.$map.find("img") as JQuery<HTMLImageElement>;

        $(window).on("resize", this.correctMapDimensions.bind(this));
    }

    // positional coordinate modifiers.
    private mapScaleMultiplier: number = 1;
    private trackScale: number = 1;
    private trackMargin: number = 0;
    private trackXOffset: number = 0;
    private trackZOffset: number = 0;

    // live map track dots
    private dots: Map<string, JQuery<HTMLElement>> = new Map<string, JQuery<HTMLElement>>();
    private maxRPMs: Map<string, number> = new Map<string, number>();

    public handleWebsocketMessage(message: WSMessage): void {
        switch (message.EventType) {
            case EventRaceControl:
                this.trackXOffset = this.raceControl.status.TrackMapData!.offset_x;
                this.trackZOffset = this.raceControl.status.TrackMapData!.offset_y;
                this.trackScale = this.raceControl.status.TrackMapData!.scale_factor;

                for (const connectedGUID in this.raceControl.status.ConnectedDrivers!.Drivers) {
                    const driver = this.raceControl.status.ConnectedDrivers!.Drivers[connectedGUID];

                    if (driver.LoadedTime.toString() !== GolangZeroTime && !this.dots.has(driver.CarInfo.DriverGUID)) {
                        // in the event that a user just loaded the race control page, place the
                        // already loaded dots onto the map
                        let $driverDot = this.buildDriverDot(driver.CarInfo, driver.LastPos as CarUpdateVector3F).show();
                        this.dots.set(driver.CarInfo.DriverGUID, $driverDot);
                    }
                }

                $(".dot").css({"transition": this.raceControl.status.CurrentRealtimePosInterval + "ms linear"});
                break;

            case EventNewConnection:
                const connectedDriver = new SessionCarInfo(message.Message);
                this.dots.set(connectedDriver.DriverGUID, this.buildDriverDot(connectedDriver));

                break;

            case EventClientLoaded:
                let carID = message.Message as number;

                if (!this.raceControl.status!.CarIDToGUID.hasOwnProperty(carID)) {
                    return;
                }

                // find the guid for this car ID:
                this.dots.get(this.raceControl.status!.CarIDToGUID[carID])!.show();

                break;

            case EventConnectionClosed:
                const disconnectedDriver = new SessionCarInfo(message.Message);
                const $dot = this.dots.get(disconnectedDriver.DriverGUID);

                if ($dot) {
                    $dot.hide();
                    this.dots.delete(disconnectedDriver.DriverGUID);
                }

                break;
            case EventCarUpdate:
                const update = new CarUpdate(message.Message);

                if (!this.raceControl.status!.CarIDToGUID.hasOwnProperty(update.CarID)) {
                    return;
                }

                // find the guid for this car ID:
                const driverGUID = this.raceControl.status!.CarIDToGUID[update.CarID];

                if (this.raceControl.status.ConnectedDrivers) {
                    let driver = this.raceControl.status.ConnectedDrivers.Drivers[driverGUID];

                    if (driver) {
                        driver.NormalisedSplinePos = update.NormalisedSplinePos;
                    }
                }

                let $myDot = this.dots.get(driverGUID);
                let dotPos = this.translateToTrackCoordinate(update.Pos);

                $myDot!.css({
                    "left": dotPos.X,
                    "top": dotPos.Z,
                });

                let speed = Math.floor(Math.sqrt((Math.pow(update.Velocity.X, 2) + Math.pow(update.Velocity.Z, 2))) * 3.6);
                let speedUnits = "Km/h";

                if (useMPH) {
                    speed = Math.floor(speed * 0.621371);
                    speedUnits = "MPH";
                }

                let maxRPM = this.maxRPMs.get(driverGUID);

                if (!maxRPM) {
                    maxRPM = 0;
                }

                if (update.EngineRPM > maxRPM) {
                    maxRPM = update.EngineRPM;
                    this.maxRPMs.set(driverGUID, update.EngineRPM);
                }

                let $rpmGaugeOuter = $("<div class='rpm-outer'></div>");
                let $rpmGaugeInner = $("<div class='rpm-inner'></div>");

                $rpmGaugeInner.css({
                    'width': ((update.EngineRPM / maxRPM) * 100).toFixed(0) + "%",
                    'background': randomColorForDriver(driverGUID),
                });

                $rpmGaugeOuter.append($rpmGaugeInner);

                let $info = $myDot!.find(".info");
                let $infoLeft = $info.find(".info-left");

                $infoLeft.text(speed + speedUnits + " " + (update.Gear - 1));
                $infoLeft.append($rpmGaugeOuter);

                // add steering angle
                let $wheel = $info.find(".steering-wheel");
                $wheel.css({
                    'transform': 'rotate(' + update.SteerAngle * 1.417 + 'deg)',
                    '-webkit-transform': 'rotate(' + update.SteerAngle * 1.417 + 'deg)',
                    '-moz-transform': 'rotate(' + update.SteerAngle * 1.417 + 'deg)',
                    '-ms-transform': 'rotate(' + update.SteerAngle * 1.417 + 'deg)',
                });

                break;

            case EventNewSession:
                this.loadTrackMapImage();

                break;

            case EventCollisionWithCar:
            case EventCollisionWithEnv:
                let collisionData = message.Message as SimpleCollision;

                let collisionMapPoint = this.translateToTrackCoordinate(collisionData.WorldPos);

                let $collision = $("<div class='collision' />").css({
                    'left': collisionMapPoint.X,
                    'top': collisionMapPoint.Z,
                });

                $collision.appendTo(this.$map);

                break;
        }
    }

    public onTrackChange(track: string, trackLayout: string): void {
        this.loadTrackMapImage();
    }

    private translateToTrackCoordinate(vec: CarUpdateVector3F): CarUpdateVector3F {
        const out = new CarUpdateVector3F();

        out.X = ((vec.X + this.trackXOffset + this.trackMargin) / this.trackScale) * this.mapScaleMultiplier;
        out.Z = ((vec.Z + this.trackZOffset + this.trackMargin) / this.trackScale) * this.mapScaleMultiplier;

        return out;
    }

    private buildDriverDot(driverData: SessionCarInfo, lastPos?: CarUpdateVector3F): JQuery<HTMLElement> {
        if (this.dots.has(driverData.DriverGUID)) {
            return this.dots.get(driverData.DriverGUID)!;
        }

        const $driverName = $("<span class='name'/>").text(driverData.DriverInitials);
        const $info = $("<span class='info'/>").hide();

        $info.append($("<div class='info-left'></div>").text("0"));
        $info.append($("<div class='steering-wheel-wrapper'><img class='steering-wheel' src='/static/img/steering-wheel.png'/></div>"));

        const $dot = $("<div class='dot' style='background: " + randomColorForDriver(driverData.DriverGUID) + "'/>").append($driverName, $info).hide().appendTo(this.$map);

        if (lastPos !== undefined) {
            let dotPos = this.translateToTrackCoordinate(lastPos);

            $dot.css({
                "left": dotPos.X,
                "top": dotPos.Z,
            });
        }

        this.dots.set(driverData.DriverGUID, $dot);

        return $dot;
    }

    private getTrackMapURL(): string {
        if (!this.raceControl.status) {
            return "";
        }

        const sessionInfo = this.raceControl.status.SessionInfo;

        return "/content/tracks/" + sessionInfo.Track + (!!sessionInfo.TrackConfig ? "/" + sessionInfo.TrackConfig : "") + "/map.png";
    }

    private loadTrackMapImage(): void {
        const trackURL = this.getTrackMapURL();
        let that = this;

        this.$trackMapImage.on("load", function () {
            that.mapImageHasLoaded = true;
            that.correctMapDimensions();
        });

        this.$trackMapImage.attr({"src": trackURL});
    }

    private static mapRotationRatio: number = 1.07;

    private correctMapDimensions(): void {
        if (!this.$trackMapImage || !this.mapImageHasLoaded) {
            return;
        }

        if (this.$trackMapImage.height()! / this.$trackMapImage.width()! > LiveMap.mapRotationRatio) {
            // rotate the map
            this.$map.addClass("rotated");

            this.$trackMapImage.css({
                'max-height': this.$trackMapImage.closest(".map-container").width()!,
                'max-width': 'auto'
            });

            this.mapScaleMultiplier = this.$trackMapImage.width()! / this.raceControl.status.TrackMapData.width;

            this.$map.closest(".map-container").css({
                'max-height': (this.raceControl.status.TrackMapData.width * this.mapScaleMultiplier) + 20,
            });

            this.$map.css({
                'max-width': (this.raceControl.status.TrackMapData.width * this.mapScaleMultiplier) + 20,
            });
        } else {
            // un-rotate the map
            this.$map.removeClass("rotated").css({
                'max-height': 'inherit',
                'max-width': '100%',
            });

            this.$map.closest(".map-container").css({
                'max-height': 'auto',
            });

            this.$trackMapImage.css({
                'max-height': 'inherit',
                'max-width': '100%'
            });

            this.mapScaleMultiplier = this.$trackMapImage.width()! / this.raceControl.status.TrackMapData.width;
        }
    }

    public getDotForDriverGUID(guid: string): JQuery<HTMLElement> | undefined {
        return this.dots.get(guid);
    }
}

const DriverGUIDDataKey = "driver-guid";

enum SessionType {
    Race = 3,
    Qualifying = 2,
    Practice = 1,
    Booking = 0,
}

enum Collision {
    WithCar = "with other car",
    WithEnvironment = "with environment",
}

class LiveTimings implements WebsocketHandler {
    private readonly raceControl: RaceControl;
    private readonly liveMap: LiveMap;

    private readonly $connectedDriversTable: JQuery<HTMLTableElement>;
    private readonly $disconnectedDriversTable: JQuery<HTMLTableElement>;
    private readonly $storedTimes: JQuery<HTMLDivElement>;

    constructor(raceControl: RaceControl, liveMap: LiveMap) {
        this.raceControl = raceControl;
        this.liveMap = liveMap;
        this.$connectedDriversTable = $("#live-table");
        this.$disconnectedDriversTable = $("#live-table-disconnected");
        this.$storedTimes = $("#stored-times");

        setInterval(this.populateConnectedDrivers.bind(this), 1000);

        $(document).on("click", ".driver-link", this.toggleDriverSpeed.bind(this));

        $(document).on("click", "#countdown", this.getFromClickEvent.bind(this));

        $(document).on("submit", "#broadcast-chat-form", this.processChatForm.bind(this));
        $(document).on("submit", "#admin-command-form", this.processAdminCommandForm.bind(this));
        $(document).on("submit", "#kick-user-form", this.processKickUserForm.bind(this));
        $(document).on("submit", "#send-chat-form", this.processSendChatForm.bind(this));

        setInterval(this.updateCarPositions.bind(this), 1000);
    }

    private updateCarPositions(): void {
        if (!this.raceControl.status.ConnectedDrivers) {
            return;
        }

        if (this.raceControl.status.SessionInfo.Type === SessionType.Race) {
            // sort drivers on the fly by their NormalisedSplinePos, if they're on the same lap.
            let oldGUIDOrder = this.raceControl.status.ConnectedDrivers.GUIDsInPositionalOrder;

            this.raceControl.status.ConnectedDrivers.GUIDsInPositionalOrder.sort((guidA: string, guidB: string): number => {
                if (!this.raceControl.status.ConnectedDrivers) {
                    return 0;
                }

                let driverA = this.raceControl.status.ConnectedDrivers.Drivers[guidA]
                let driverB = this.raceControl.status.ConnectedDrivers.Drivers[guidB]

                if (driverA.TotalNumLaps === driverB.TotalNumLaps) {
                    if (driverA.NormalisedSplinePos > driverB.NormalisedSplinePos) {
                        return -1;
                    } else if (driverA.NormalisedSplinePos < driverB.NormalisedSplinePos) {
                        return 1;
                    } else {
                        return 0;
                    }
                }

                // javascript has no stable sort. use our (backend sorted) position numbers to ensure stability.
                return driverA.Position - driverB.Position;
            });

            let guidOrderHasChanged = false;
            let newGUIDOrder = this.raceControl.status.ConnectedDrivers.GUIDsInPositionalOrder;

            if (oldGUIDOrder.length !== newGUIDOrder.length) {
                guidOrderHasChanged = true;
            } else {
                for (let i = 0; i < oldGUIDOrder.length; i++) {
                    if (oldGUIDOrder[i] !== newGUIDOrder[i]) {
                        guidOrderHasChanged = true;
                        break;
                    }
                }
            }

            if (guidOrderHasChanged) {
                this.refresh();
            }
        }
    }

    private getFromClickEvent(e: ClickEvent): void {
        e.preventDefault();
        e.stopPropagation();

        const $target = $(e.currentTarget);
        const href = $target.attr("href");

        $.get(href);
    }

    private processChatForm(e: JQuery.SubmitEvent): boolean {
        this.postForm(e);

        $(".broadcast-chat").val('');

        return false
    }

    private processSendChatForm(e: JQuery.SubmitEvent): boolean {
        this.postForm(e);

        $(".send-chat").val('');

        return false
    }

    private processAdminCommandForm(e: JQuery.SubmitEvent): boolean {
        this.postForm(e);

        $(".admin-command").val('');

        return false
    }

    private processKickUserForm(e: JQuery.SubmitEvent): boolean {
        this.postForm(e);

        return false
    }

    private postForm(e: JQuery.SubmitEvent) {
        e.preventDefault();
        e.stopPropagation();

        this.post($(e.currentTarget));
    }

    private post(form: JQuery<HTMLFormElement>) {
        $.ajax({
            url: form.attr("action"),
            type: 'post',
            data: form.serialize(),
            success:function(){

            }
        });
    }

    public refresh(): void {
        this.populateConnectedDrivers();
    }

    public handleWebsocketMessage(message: WSMessage): void {
        if (message.EventType === EventRaceControl) {
            this.populateConnectedDrivers();
            this.initialiseAdminSelects();
            this.populateDisconnectedDrivers();
        } else if (message.EventType === EventConnectionClosed) {
            const closedConnection = message.Message as SessionCarInfo;

            this.removeDriverFromAdminSelects(closedConnection);

            if (this.raceControl.status.ConnectedDrivers) {
                const driver = this.raceControl.status.ConnectedDrivers.Drivers[closedConnection.DriverGUID];

                if (driver && (driver.LoadedTime.toString() === GolangZeroTime || !driver.TotalNumLaps)) {
                    // a driver joined but never loaded, or hasn't completed any laps. remove them from the connected drivers table.
                    this.$connectedDriversTable.find("tr[data-guid='" + closedConnection.DriverGUID + "']").remove();
                    this.removeDriverFromAdminSelects(driver.CarInfo)
                }
            }
        } else if (message.EventType === EventNewConnection) {
            const connectedDriver = new SessionCarInfo(message.Message);

            this.addDriverToAdminSelects(connectedDriver);
        } else if (message.EventType === EventTyresChanged) {
            const driver = new SessionCarInfo(message.Message);

            this.onTyreChange(driver);
        }
    }

    public onTrackChange(track: string, trackLayout: string): void {

    }

    private populateConnectedDrivers(): void {
        if (!this.raceControl.status || !this.raceControl.status.ConnectedDrivers) {
            return;
        }

        let position = 1;

        for (const driverGUID of this.raceControl.status.ConnectedDrivers.GUIDsInPositionalOrder) {
            const driver = this.raceControl.status.ConnectedDrivers.Drivers[driverGUID];

            if (!driver) {
                continue;
            }

            driver.Position = position;

            this.addDriverToTable(driver, this.$connectedDriversTable);
            this.populatePreviousLapsForDriver(driver);

            position++;
        }
    }

    private populatePreviousLapsForDriver(driver: Driver): void {
        for (const carName in driver.Cars) {
            if (carName === driver.CarInfo.CarModel) {
                continue;
            }

            // create a fake new driver from the old driver. override details with their previous car
            // and add them to the disconnected drivers table. if the user rejoins in this car it will
            // be removed from the disconnected drivers table and placed into the connected drivers table.
            const dummyDriver = JSON.parse(JSON.stringify(driver));
            dummyDriver.CarInfo.CarModel = carName;
            dummyDriver.CarInfo.CarName = driver.Cars[carName].CarName;

            this.addDriverToTable(dummyDriver, this.$disconnectedDriversTable);
        }
    }

    private populateDisconnectedDrivers(): void {
        if (!this.raceControl.status || !this.raceControl.status.DisconnectedDrivers) {
            return;
        }

        for (const driverGUID of this.raceControl.status.DisconnectedDrivers.GUIDsInPositionalOrder) {
            const driver = this.raceControl.status.DisconnectedDrivers.Drivers[driverGUID];

            if (!driver) {
                continue;
            }

            this.addDriverToTable(driver, this.$disconnectedDriversTable);
            this.populatePreviousLapsForDriver(driver);
        }

        if (this.$disconnectedDriversTable.find("tr").length > 1) {
            this.$storedTimes.show();
        } else {
            this.$storedTimes.hide();
        }
    }

    private static CONNECTED_ROW_HTML = `
        <tr class="driver-row">
            <td class="driver-pos text-center"></td>
            <td class="driver-name driver-link"></td>
            <td class="driver-car"></td>
            <td class="current-tyres"></td>
            <td class="current-lap"></td>
            <td class="last-lap"></td>
            <td class="best-lap"></td>
            <td class="gap"></td>
            <td class="num-laps"></td>
            <td class="top-speed"></td>
        </tr>
    `;

    private static DISCONNECTED_ROW_HTML = `
        <tr class="driver-row">
            <td class="driver-name"></td>
            <td class="driver-car"></td>
            <td class="best-lap"></td>
            <td class="num-laps"></td>
            <td class="top-speed"></td>
        </tr>
    `;

    private static DAMAGE_ZONES = `
        <svg
           class="damage-zone-svg"
           xmlns:dc="http://purl.org/dc/elements/1.1/"
           xmlns:cc="http://creativecommons.org/ns#"
           xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
           xmlns:svg="http://www.w3.org/2000/svg"
           xmlns="http://www.w3.org/2000/svg"
           xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd"
           xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"
           width="174"
           height="89.485718"
           id="svg2"
           version="1.1">
          <defs
             id="defs4" />
          <sodipodi:namedview
             id="base"
             pagecolor="#ffffff"
             bordercolor="#666666"
             borderopacity="1.0"
             inkscape:pageopacity="0.0"
             inkscape:pageshadow="2"
             inkscape:zoom="2.8"
             inkscape:cx="37.633141"
             inkscape:cy="96.538946"
             inkscape:document-units="px"
             inkscape:current-layer="layer1"
             showgrid="false"
             showguides="false"
             inkscape:guide-bbox="true"
             inkscape:window-width="2009"
             inkscape:window-height="1160"
             inkscape:window-x="565"
             inkscape:window-y="0"
             inkscape:window-maximized="0"
             fit-margin-top="0"
             fit-margin-left="0"
             fit-margin-right="0"
             fit-margin-bottom="0">
            <sodipodi:guide
               orientation="1,0"
               position="53.907558,436.45116"
               id="guide3877"
               inkscape:locked="false" />
            <sodipodi:guide
               orientation="1,0"
               position="-152.26223,-297.6478"
               id="guide3938"
               inkscape:locked="false" />
            <sodipodi:guide
               orientation="1,0"
               position="263.8092,-296.57637"
               id="guide3940"
               inkscape:locked="false" />
          </sodipodi:namedview>
          <metadata
             id="metadata7">
            <rdf:RDF>
              <cc:Work
                 rdf:about="">
                <dc:format>image/svg+xml</dc:format>
                <dc:type
                   rdf:resource="http://purl.org/dc/dcmitype/StillImage" />
                <dc:title></dc:title>
              </cc:Work>
            </rdf:RDF>
          </metadata>
          <g
             id="layer1"
             transform="translate(-318.33366,-463.80011)">
            <path
               style="color:#000000;display:inline;overflow:visible;visibility:visible;fill:#ff0000;fill-opacity:1;fill-rule:nonzero;stroke:none;marker:none;enable-background:accumulate"
               class="front-bumper"
               d="m 475.24297,465.87983 c 5.14878,3.61944 10.95993,10.20237 13.98704,15.84467 1.72874,3.22225 2.30497,5.21415 2.60016,8.98821 0.3896,4.98092 0.50429,8.9516 0.50348,17.42881 -9.9e-4,8.49589 -0.12265,12.7603 -0.50385,17.64598 -0.29442,3.77281 -0.8708,5.76546 -2.59979,8.98821 -2.69115,5.01607 -7.39119,10.59645 -12.1854,14.46777 -1.54534,1.24786 -2.56444,1.95085 -2.5634,1.76826 4.4e-4,-0.077 0.0479,-0.55515 0.10532,-1.06281 0.0576,-0.50766 0.20072,-2.0225 0.31825,-3.3663 1.64142,-18.76882 2.06147,-38.56613 1.22574,-57.77026 -0.32898,-7.55922 -1.15237,-19.31444 -1.60788,-22.95475 l -0.0663,-0.53077 0.78667,0.55298 z"
               id="path3900" />
            <path
               style="fill:#ff0000;fill-opacity:1;stroke:none"
               class="left-skirt"
               d="m 444.64627,464.42958 c 0,0 -7.10155,6.7571 -17.67834,6.7571 -10.57679,0 -50.16418,-2.30357 -53.03501,-2.76427 -2.87085,-0.46071 -8.00814,-4.91426 -9.21692,-4.60713 -1.20877,0.30715 -1.02533,1.17493 -1.02533,3.88923 0,2.71432 0,3.94288 0,3.94288 0,0 -0.48563,1.22857 2.08302,1.68927 2.56864,0.46072 56.81243,0.61428 64.21618,0.76785 7.40374,0.1536 11.33226,1.38213 12.38994,-2.30355 1.05769,-3.68569 5.4395,-7.67853 2.26646,-7.37138 z"
               id="path3768" />
            <path
               style="color:#000000;display:inline;overflow:visible;visibility:visible;fill:#ff0000;fill-opacity:1;fill-rule:nonzero;stroke:none;marker:none;enable-background:accumulate"
               class="rear-bumper"
               d="m 318.4842,478.34628 c 0.35991,-7.62531 1.28961,-10.02709 4.67963,-12.08919 2.13723,-1.30005 6.48405,-2.24956 10.31021,-2.25215 l 1.86973,-10e-4 v 44.63331 44.63332 l -0.80131,-1.8e-4 c -1.32146,-2.3e-4 -5.42644,-0.41286 -6.57076,-0.66038 -3.13948,-0.67907 -5.24096,-1.67342 -6.68952,-3.16527 -1.5968,-1.64449 -2.2023,-3.35444 -2.63131,-7.43086 -0.30353,-2.88436 -0.44712,-57.7256 -0.16667,-63.66734 z"
               id="path3936" />
            <path
               style="fill:#424242;fill-opacity:1;stroke:none"
               class="left-rear-tyre"
               d="m 336.91318,477.34011 c 0,0 0,2.21102 0,3.18171 0,2.5885 0.12567,2.64242 3.6436,2.64242 4.27181,0 16.55989,0 18.72059,0 2.40116,0 3.33505,0.68481 3.33505,-1.6529 0,-2.33771 0,-14.74095 0,-15.92736 0,-1.18641 0.29796,-1.62439 -3.27754,-1.62439 -3.57548,0 -16.83065,0 -19.21784,0 -2.38718,0 -3.08204,0.64712 -3.08204,2.64571 0,1.9986 -0.12185,10.73481 -0.12185,10.73481 z"
               id="path3790" />
            <path
               style="fill:#424242;fill-opacity:1;stroke:none"
               class="left-front-tyre"
               d="m 446.87737,477.63968 c 0,0 0,2.21101 0,3.18171 0,2.5885 0.12567,2.64244 3.64361,2.64244 4.27181,0 16.55988,0 18.72058,0 2.40116,0 3.33505,0.68481 3.33505,-1.65291 0,-2.33771 0,-14.74097 0,-15.92737 0,-1.18639 0.29796,-1.6244 -3.27753,-1.6244 -3.57549,0 -16.83066,0 -19.21785,0 -2.38718,0 -3.08204,0.64714 -3.08204,2.64574 0,1.99858 -0.12185,10.73479 -0.12185,10.73479 z"
               id="path3790-3" />
            <path
               style="fill:#424242;fill-opacity:1;stroke:none"
               class="right-front-tyre"
               d="m 446.87737,539.42805 c 0,0 0,-2.21103 0,-3.18171 0,-2.58852 0.12567,-2.64244 3.64361,-2.64244 4.27181,0 16.55988,0 18.72058,0 2.40116,0 3.33505,-0.6848 3.33505,1.65291 0,2.33771 0,14.74097 0,15.92737 0,1.1864 0.29796,1.62438 -3.27753,1.62438 -3.57549,0 -16.83066,0 -19.21785,0 -2.38718,0 -3.08204,-0.64712 -3.08204,-2.64572 0,-1.9986 -0.12185,-10.73479 -0.12185,-10.73479 z"
               id="path3790-3-7" />
            <path
               style="fill:#424242;fill-opacity:1;stroke:none"
               class="right-rear-tyre"
               d="m 336.91318,539.90531 c 0,0 0,-2.21102 0,-3.1817 0,-2.58851 0.12567,-2.64243 3.6436,-2.64243 4.27181,0 16.55989,0 18.72059,0 2.40116,0 3.33505,-0.68481 3.33505,1.6529 0,2.33771 0,14.74097 0,15.92737 0,1.1864 0.29796,1.62438 -3.27754,1.62438 -3.57548,0 -16.83065,0 -19.21784,0 -2.38718,0 -3.08204,-0.64712 -3.08204,-2.64571 0,-1.9986 -0.12185,-10.73481 -0.12185,-10.73481 z"
               id="path3790-2" />
            <path
               style="fill:#ff0000;fill-opacity:1;stroke:none;stroke-width:0.49714285"
               class="right-skirt"
               d="m 444.64627,552.6558 c 0,0 -7.10155,-6.75711 -17.67834,-6.75711 -10.57679,0 -50.16418,2.30357 -53.03501,2.76428 -2.87085,0.4607 -8.00814,4.91425 -9.21692,4.60712 -1.20877,-0.30715 -1.02533,-1.17493 -1.02533,-3.88923 0,-2.71432 0,-3.94287 0,-3.94287 0,0 -0.48563,-1.22856 2.08302,-1.68928 2.56864,-0.46072 56.81243,-0.61428 64.21618,-0.76785 7.40374,-0.1536 11.33226,-1.38213 12.38994,2.30355 1.05769,3.68569 5.4395,7.67853 2.26646,7.37139 z"
               id="path3768-1" />
          </g>
        </svg>

    `;

    private newRowForDriver(driver: Driver, addingToConnectedTable: boolean): JQuery<HTMLElement> {
        const $tr = $(addingToConnectedTable ? LiveTimings.CONNECTED_ROW_HTML : LiveTimings.DISCONNECTED_ROW_HTML);
        $tr.attr({
            "data-guid": driver.CarInfo.DriverGUID,
            "data-car-model": driver.CarInfo.CarModel,
        });

        const $tdName = $tr.find(".driver-name");
        $tdName.text(driver.CarInfo.DriverName);

        if (addingToConnectedTable) {
            // driver dot
            const driverDot = this.liveMap.getDotForDriverGUID(driver.CarInfo.DriverGUID);

            if (driverDot) {
                let dotClass = "dot";

                if (driverDot.find(".info").is(":hidden")) {
                    dotClass += " dot-inactive";
                }

                $tdName.prepend($("<div/>").attr({"class": dotClass}).css("background", randomColor({
                    luminosity: 'bright',
                    seed: driver.CarInfo.DriverGUID,
                })));
            }

            $tdName.attr("class", "driver-link");
            $tdName.data(DriverGUIDDataKey, driver.CarInfo.DriverGUID);
        }

        return $tr;
    }

    private addDriverToTable(driver: Driver, $table: JQuery<HTMLTableElement>): void {
        const addingDriverToConnectedTable = ($table === this.$connectedDriversTable);
        const carInfo = driver.Cars[driver.CarInfo.CarModel];

        if (!carInfo) {
            return;
        }

        let $tr = $table.find("[data-guid='" + driver.CarInfo.DriverGUID + "'][data-car-model='"+ driver.CarInfo.CarModel + "']");

        let addTrToTable = false;

        if (!$tr.length) {
            addTrToTable = true;
            $tr = this.newRowForDriver(driver, addingDriverToConnectedTable) as JQuery<HTMLTableElement>;
        }

        // car position
        if (addingDriverToConnectedTable) {
            $tr.find(".driver-pos").text(driver.Position === 255 || driver.Position === 0 ? "" : driver.Position);
        }

        // car model
        $tr.find(".driver-car").text(carInfo.CarName ? carInfo.CarName : prettifyName(driver.CarInfo.CarModel, true));

        if (addingDriverToConnectedTable) {
            // tyres
            $tr.find(".current-tyres").text(driver.CarInfo.Tyres);

            let currentLapTimeText = "";

            if (driver.LoadedTime.toString() !== GolangZeroTime) {
                if (moment(carInfo.LastLapCompletedTime).utc().isSameOrAfter(moment(this.raceControl.status!.SessionStartTime).utc())) {
                    // only show current lap time text if the last lap completed time is after session start.
                    currentLapTimeText = msToTime(moment().utc().diff(moment(carInfo.LastLapCompletedTime).utc()), false, 1);
                }
            }

            let $currentLap = $tr.find(".current-lap");

            $currentLap.text(currentLapTimeText);

            for (const splitIndex in carInfo.CurrentLapSplits) {
                let split = carInfo.CurrentLapSplits[splitIndex];

                let $tag = $("<span/>");

                let badgeColour = "warning";

                if (split.IsDriversBest !== undefined && split.IsDriversBest) {
                    badgeColour = "success";
                }

                if (split.IsBest !== undefined && split.IsBest) {
                    badgeColour = "info";
                }

                if (split.Cuts !== undefined && split.Cuts !== 0) {
                    badgeColour = "danger";
                }

                $tag.attr({'id': `split-` + splitIndex, 'class': 'badge ml-2 mt-1 badge-' + badgeColour});

                if (split.SplitIndex === undefined) {
                    split.SplitIndex = 0
                }

                $tag.text(
                    "S" + (split.SplitIndex+1) + ": " + msToTime(split.SplitTime / 1000000, true, 2)
                );

                $currentLap.append($tag);
            }
        }

        if (addingDriverToConnectedTable) {
            // last lap
            $tr.find(".last-lap").text(msToTime(carInfo.LastLap / 1000000));
        }

        // best lap
        $tr.find(".best-lap").text(msToTime(carInfo.BestLap / 1000000) + (carInfo.TyreBestLap ? " (" + carInfo.TyreBestLap + ")" : ""));

        if (addingDriverToConnectedTable) {
            // gap
            $tr.find(".gap").text(driver.Split);
        }

        // lap number
        $tr.find(".num-laps").text(carInfo.NumLaps ? carInfo.NumLaps : "0");

        let topSpeed;
        let speedUnits;

        if (useMPH) {
            topSpeed = carInfo.TopSpeedBestLap * 0.621371;
            speedUnits = "MPH";
        } else {
            topSpeed = carInfo.TopSpeedBestLap;
            speedUnits = "Km/h";
        }

        $tr.find(".top-speed").text(topSpeed ? topSpeed.toFixed(2) + speedUnits : "");

        if (addingDriverToConnectedTable) {
            let $shownToasts = $('.damage-zone-toast:visible');

            if ($shownToasts.length > 6) {
                for (let i = 6; i < $shownToasts.length;  i++) {
                    $($shownToasts[i]).toast('hide');
                }
            }

            // events
            if (driver.Collisions) {
                for (const collision of driver.Collisions) {
                    const collisionID = driver.CarInfo.DriverGUID + "-collision-" + collision.ID;

                    if (moment(collision.Time).utc().add("10", "seconds").isSameOrAfter(moment().utc()) && !$("#" + collisionID).length) {

                        let crashSpeed;

                        if (useMPH) {
                            crashSpeed = collision.Speed * 0.621371;
                        } else {
                            crashSpeed = collision.Speed;
                        }

                        let $toast = $("<div/>");
                        let $toastHeader = $("<div/>");
                        let $toastBody = $("<div/>");

                        let toastHeaderBG = "";
                        let toastHeaderBD = "";

                        if (collision.Type === Collision.WithCar) {
                            toastHeaderBG = "bg-danger";
                            toastHeaderBD = "border-danger";

                            $toastHeader.text(
                                "Crash with " + collision.OtherDriverName
                            );
                        } else {
                            toastHeaderBG = "bg-warning";
                            toastHeaderBD = "border-warning";

                            $toastHeader.text(
                                "Crash " + collision.Type
                            );
                        }

                        $toast.attr({'id': collisionID, 'class': 'toast damage-zone-toast', 'role': 'alert', 'aria-live': 'assertive', 'aria-atomic': 'true'});
                        $toastHeader.attr('class', 'toast-header damage-zone-toast-header ' + toastHeaderBG + ' ' + toastHeaderBD);
                        $toastBody.attr('class', 'toast-body damage=zone-toast-body');

                        let $textContainer = $("<div/>");
                        $textContainer.attr('style', 'height: 18px')

                        let $driverName = $("<strong/>");
                        $driverName.text(driver.CarInfo.DriverName + ' ');

                        let $crashSpeed = $("<span/>");
                        $crashSpeed.attr('class', 'text-secondary');
                        $crashSpeed.text(crashSpeed.toFixed(2) + speedUnits);

                        $textContainer.append($driverName);
                        $textContainer.append($crashSpeed);

                        $toastBody.append($textContainer);

                        let $damageZones = $(LiveTimings.DAMAGE_ZONES);

                        let frontBumperHue = 70 - collision.DamageZones[0];
                        let rearBumperHue = 70 - collision.DamageZones[1];
                        let leftSkirtHue = 70 - collision.DamageZones[2];
                        let rightSkirtHue = 70 - collision.DamageZones[3];

                        if (frontBumperHue < 0) {
                            frontBumperHue = 0
                        }

                        if (rearBumperHue < 0) {
                            rearBumperHue = 0
                        }

                        if (leftSkirtHue < 0) {
                            leftSkirtHue = 0
                        }

                        if (rightSkirtHue < 0) {
                            rightSkirtHue = 0
                        }

                        $damageZones.find(".front-bumper").attr("style", "fill: hsl(" + frontBumperHue + ", 100%, 50%);fill-opacity:1;stroke:none");
                        $damageZones.find(".rear-bumper").attr("style", "fill: hsl(" + rearBumperHue + ", 100%, 50%);fill-opacity:1;stroke:none");
                        $damageZones.find(".left-skirt").attr("style", "fill: hsl(" + leftSkirtHue + ", 100%, 50%);fill-opacity:1;stroke:none");
                        $damageZones.find(".right-skirt").attr("style", "fill: hsl(" + rightSkirtHue + ", 100%, 50%);fill-opacity:1;stroke:none");

                        $toastBody.append($damageZones);

                        $toast.append($toastHeader);
                        $toast.append($toastBody);

                        $toast.toast({animation: true, autohide: true, delay: 10000});
                        $toast.toast('show');

                        $("#toasts").append($toast);

                        setTimeout(() => {
                            $toast.remove();
                        }, 12000);
                    }
                }
            }
        }

        if (!addingDriverToConnectedTable) {
            // if we're adding to the disconnected table, ensure we've removed this driver and car from the connected table.
            this.$connectedDriversTable.find("[data-guid='" + driver.CarInfo.DriverGUID + "'][data-car-model='" + driver.CarInfo.CarModel + "']").remove();
        } else {
            // remove the driver from the disconnected table
            this.$disconnectedDriversTable.find("[data-guid='" + driver.CarInfo.DriverGUID + "'][data-car-model='" + driver.CarInfo.CarModel + "']").remove();
        }

        if (!addingDriverToConnectedTable && (!carInfo.NumLaps || carInfo.NumLaps === 0)) {
            return;
        }

        if (addTrToTable) {
            $table.append($tr);
        } else {
            if (driver.Position > 0 && addingDriverToConnectedTable) {
                $table.find("tr").eq(driver.Position - 1).after($tr.detach());
            }
        }

        if (!addingDriverToConnectedTable) {
            this.sortDisconnectedTable($table);
        }
    }

    private sortDisconnectedTable($table: JQuery<HTMLTableElement>) {
        const $tbody = $table.find("tbody");
        const that = this;

        $($tbody.find("tr:not(:nth-child(1))").get().sort(function (a: HTMLTableElement, b: HTMLTableElement): number {
            if (that.raceControl.status.SessionInfo.Type === SessionType.Race) {
                let lapsA = parseInt($(a).find("td:nth-child(4)").text(), 10);
                let lapsB = parseInt($(b).find("td:nth-child(4)").text(), 10);

                if (lapsA !== 0 && lapsB !== 0 && lapsA < lapsB) {
                    return 1;
                } else if (lapsA === lapsB) {
                    return 0;
                } else {
                    return -1;
                }
            } else {
                let timeA = $(a).find("td:nth-child(3)").text();
                let timeB = $(b).find("td:nth-child(3)").text();

                if (timeA !== "" && timeB !== "" && timeA < timeB) {
                    return -1;
                } else if (timeA === timeB) {
                    return 0;
                } else if (timeA === "") {
                    return 1; // sort a to the back
                } else if (timeB === "") {
                    return -1; // sort b to the back
                } else {
                    return 1; // B < A && timeA != "" && timeB != ""
                }
            }
        })).appendTo($tbody);
    }

    private toggleDriverSpeed(e: ClickEvent): void {
        const $target = $(e.currentTarget);
        const driverGUID = $target.data(DriverGUIDDataKey);
        const $driverDot = this.liveMap.getDotForDriverGUID(driverGUID);

        if (!$driverDot) {
            return;
        }

        $driverDot.find(".info").toggle();
        $target.find(".dot").toggleClass("dot-inactive");
    }

    private onTyreChange(driver: SessionCarInfo): void {
        let $tr = this.$connectedDriversTable.find("[data-guid='" + driver.DriverGUID + "'][data-car-model='"+ driver.CarModel + "']");

        if (!$tr.length) {
            return;
        }

        if (this.raceControl.status.ConnectedDrivers) {
            const connectedDriver = this.raceControl.status.ConnectedDrivers.Drivers[driver.DriverGUID];

            if (connectedDriver) {
                connectedDriver.CarInfo.Tyres = driver.Tyres;
            }
        }

        $tr.find(".current-tyres").text(driver.Tyres);
    }

    private initialisedAdmin = false;

    private initialiseAdminSelects() {
        if (this.initialisedAdmin) {
            return
        }

        if (!this.raceControl.status || !this.raceControl.status.ConnectedDrivers) {
            return;
        }

        for (const driverGUID of this.raceControl.status.ConnectedDrivers.GUIDsInPositionalOrder) {
            const driver = this.raceControl.status.ConnectedDrivers.Drivers[driverGUID];

            if (!driver) {
                continue;
            }

            this.addDriverToAdminSelects(driver.CarInfo);
        }

        this.initialisedAdmin = true
    }

    private addDriverToAdminSelects(carInfo: SessionCarInfo) {
        $(".kick-user option[value='default-driver-spacer']").remove();
        $(".chat-user option[value='default-driver-spacer']").remove();

        if ($(".kick-user option[value=" + carInfo.DriverGUID + "]").length != 0) {
            // driver already exists
        } else {
            // add driver to admin kick list
            $('.kick-user').append($('<option>', {
                value: carInfo.DriverGUID,
                text: carInfo.DriverName,
            }));
        }

        if ($(".chat-user option[value=" + carInfo.DriverGUID + "]").length != 0) {
            // driver already exists
        } else {
            // add driver to admin kick list
            $('.chat-user').append($('<option>', {
                value: carInfo.DriverGUID,
                text: carInfo.DriverName,
            }));
        }
    }

    private removeDriverFromAdminSelects(carInfo: SessionCarInfo) {
        $(".kick-user option[value=" + carInfo.DriverGUID + "]").remove();
        $(".chat-user option[value=" + carInfo.DriverGUID + "]").remove();
    }
}

function randomColorForDriver(driverGUID: string): string {
    return randomColor({
        seed: driverGUID,
    })
}

const percentColors = [
    {pct: 0.25, color: {r: 0x00, g: 0x00, b: 0xff}},
    {pct: 0.625, color: {r: 0x00, g: 0xff, b: 0}},
    {pct: 1.0, color: {r: 0xff, g: 0x00, b: 0}}
];

function getColorForPercentage(pct: number): string {
    let i;

    for (i = 1; i < percentColors.length - 1; i++) {
        if (pct < percentColors[i].pct) {
            break;
        }
    }

    let lower = percentColors[i - 1];
    let upper = percentColors[i];
    let range = upper.pct - lower.pct;
    let rangePct = (pct - lower.pct) / range;
    let pctLower = 1 - rangePct;
    let pctUpper = rangePct;
    let color = {
        r: Math.floor(lower.color.r * pctLower + upper.color.r * pctUpper),
        g: Math.floor(lower.color.g * pctLower + upper.color.g * pctUpper),
        b: Math.floor(lower.color.b * pctLower + upper.color.b * pctUpper)
    };

    return 'rgb(' + [color.r, color.g, color.b].join(',') + ')';
}
