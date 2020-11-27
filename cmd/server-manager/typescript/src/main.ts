import "jquery";
import "bootstrap";
import "bootstrap-switch";
import "summernote/dist/summernote-bs4";
import "multiselect";
import "moment";
import "moment-timezone";
import "./javascript/manager.js";
import "./Font";
import "./Calendar";

import {RaceControl} from "./RaceControl";
import {CarDetail} from "./CarDetail";
import {TrackDetail} from "./TrackDetail";
import {CarSearch} from "./CarSearch";
import {CarList} from "./CarList";
import {RaceWeekend} from "./RaceWeekend";
import {ChangelogPopup} from "./ChangelogPopup";
import {HostedIntroPopup} from "./HostedIntroPopup";
import {Messages} from "./Messages";
import {Championship} from "./Championship";
import {CustomRace} from "./CustomRace"
import {Results} from "./Results";
import {RaceList} from "./RaceList";
import {SpectatorCar} from "./SpectatorCar";
import {CustomChecksums} from "./CustomChecksums";

$(() => {
    new RaceControl();
    new CarDetail();
    new TrackDetail();
    new CarList();
    new RaceWeekend.View();
    new RaceWeekend.EditSession();
    new ChangelogPopup();
    new HostedIntroPopup();
    Messages.initSummerNote();
    new Championship.View();
    new CustomRace.Edit();
    new Results();
    new RaceList();
    new SpectatorCar();
    new CustomChecksums();

    $(".race-setup").each(function (index, elem) {
        new CarSearch($(elem));
    });
});

declare global {
    interface JQuery {
        multiSelect: any;
    }
}
