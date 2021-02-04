import dragula from "dragula";

export namespace CustomRace {
    export class Edit {
        public constructor() {
            this.initDraggableCards();
            this.initPenaltyTypeWatcher();
            this.initShowEnabledPenaltyTypes();
        }

        private initDraggableCards(): void {
            let drake = dragula([document.querySelector(".weather-container")!], {
                moves: (el?: Element, source?: Element, handle?: Element, sibling?: Element): boolean => {
                    if (!handle) {
                        return false;
                    }

                    return $(handle).hasClass("card-header");
                },
            });

            drake.on("drop", () => {
                $(".weather-num").each(function (index) {
                    $(this).text(index);
                });

                let $weatherDelete = $(".weather-delete");

                $weatherDelete.show();
                $weatherDelete.first().hide();
            });
        }
        private initShowEnabledPenaltyTypes(): void {
            $(".penalty-type-enabler").each(function (index, elem) {
                $(elem).on('switchChange.bootstrapSwitch', function (event, state) {
                    let $this = $(this);
                    let $panelLabel = $("#" + $this.closest(".tab-pane").attr("aria-labelledby"));

                    if (state) {
                        $panelLabel.addClass("text-success");
                    } else {
                        $panelLabel.removeClass("text-success");
                    }
                });
            });
        }

        private initPenaltyTypeWatcher(): void {
            let $penaltyType = $("#CustomCutsPenaltyType");
            let $penaltyTypeCollision = $("#CollisionPenaltiesPenaltyType");
            let $penaltyTypeDRS = $("#DRSPenaltiesPenaltyType");

            if (!$penaltyType) {
                return;
            }

            $penaltyType.on("change", function (e) {
                let $this = $(e.currentTarget) as JQuery<HTMLInputElement>;
                let value = $this.val() as number;

                let $customCutsBoPAmountWrapper = $("#CustomCutsBoPAmountWrapper");
                let $customCutsBoPNumLapsWrapper = $("#CustomCutsBoPNumLapsWrapper");

                let $customCutsBoPAmount = $("#CustomCutsBoPAmount");
                let $customCutsBoPNumLaps = $("#CustomCutsBoPNumLaps");

                if (value == 1 || value == 2) {
                    $customCutsBoPAmountWrapper.show();
                    $customCutsBoPNumLapsWrapper.show();

                    $customCutsBoPAmount.attr("min", "0");
                    $customCutsBoPNumLaps.attr("min", "1");
                } else {
                    $customCutsBoPAmountWrapper.hide();
                    $customCutsBoPNumLapsWrapper.hide();

                    $customCutsBoPAmount.attr("min", "");
                    $customCutsBoPNumLaps.attr("min", "");
                }

                if (value == 4) {
                    $("#CustomCutsDriveThroughNumLapsWrapper").show();
                } else {
                    $("#CustomCutsDriveThroughNumLapsWrapper").hide();
                }
            });

            $penaltyTypeCollision.on("change", function (e) {
                let $this = $(e.currentTarget) as JQuery<HTMLInputElement>;
                let value = $this.val() as number;

                let $collisionPenaltiesBoPAmountWrapper = $("#CollisionPenaltiesBoPAmountWrapper");
                let $collisionPenaltiesBoPNumLapsWrapper = $("#CollisionPenaltiesBoPNumLapsWrapper");

                let $collisionPenaltiesBoPAmount = $("#CollisionPenaltiesBoPAmount");
                let $collisionPenaltiesBoPNumLaps = $("#CollisionPenaltiesBoPNumLaps");

                if (value == 1 || value == 2) {
                    $collisionPenaltiesBoPAmountWrapper.show();
                    $collisionPenaltiesBoPNumLapsWrapper.show();

                    $collisionPenaltiesBoPAmount.attr("min", "0");
                    $collisionPenaltiesBoPNumLaps.attr("min", "1");
                } else {
                    $collisionPenaltiesBoPAmountWrapper.hide();
                    $collisionPenaltiesBoPNumLapsWrapper.hide();

                    $collisionPenaltiesBoPAmount.attr("min", "");
                    $collisionPenaltiesBoPNumLaps.attr("min", "");
                }

                if (value == 4) {
                    $("#CollisionPenaltiesDriveThroughNumLapsWrapper").show();
                } else {
                    $("#CollisionPenaltiesDriveThroughNumLapsWrapper").hide();
                }
            });

            $penaltyTypeDRS.on("change", function (e) {
                let $this = $(e.currentTarget) as JQuery<HTMLInputElement>;
                let value = $this.val() as number;

                let $drsPenaltiesBoPAmountWrapper = $("#DRSPenaltiesBoPAmountWrapper");
                let $drsPenaltiesBoPNumLapsWrapper = $("#DRSPenaltiesBoPNumLapsWrapper");

                let $drsPenaltiesBoPAmount = $("#DRSPenaltiesBoPAmount");
                let $drsPenaltiesBoPNumLaps = $("#DRSPenaltiesBoPNumLaps");

                if (value == 1 || value == 2) {
                    $drsPenaltiesBoPAmountWrapper.show();
                    $drsPenaltiesBoPNumLapsWrapper.show();

                    $drsPenaltiesBoPAmount.attr("min", "0");
                    $drsPenaltiesBoPNumLaps.attr("min", "1");
                } else {
                    $drsPenaltiesBoPAmountWrapper.hide();
                    $drsPenaltiesBoPNumLapsWrapper.hide();

                    $drsPenaltiesBoPAmount.attr("min", "");
                    $drsPenaltiesBoPNumLaps.attr("min", "");
                }

                if (value == 4) {
                    $("#DRSPenaltiesDriveThroughNumLapsWrapper").show();
                } else {
                    $("#DRSPenaltiesDriveThroughNumLapsWrapper").hide();
                }
            });
        }
    }
}
