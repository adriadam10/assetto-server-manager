import dragula from "dragula";

export namespace CustomRace {
    export class View {
        public constructor() {
            this.initDraggableCards();
            this.initPenaltyTypeWatcher();
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

        private initPenaltyTypeWatcher(): void {
            let $penaltyType = $("#CustomCutsPenaltyType");

            if (!$penaltyType) {
                return;
            }

            $penaltyType.on("change", function (e) {
                let $this = $(e.currentTarget) as JQuery<HTMLInputElement>;
                let value = $this.val() as number;

                if (value == 1 || value == 2) {
                    $("#CustomCutsBoPAmountWrapper").show();
                    $("#CustomCutsBoPNumLapsWrapper").show();
                } else {
                    $("#CustomCutsBoPAmountWrapper").hide();
                    $("#CustomCutsBoPNumLapsWrapper").hide();
                }

                if (value == 4) {
                    $("#CustomCutsDriveThroughNumLapsWrapper").show();
                } else {
                    $("#CustomCutsDriveThroughNumLapsWrapper").hide();
                }
            })
        }
    }
}
