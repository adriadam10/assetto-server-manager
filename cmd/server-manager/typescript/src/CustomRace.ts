import dragula from "dragula";

export namespace CustomRace {
    export class View {
        public constructor() {
            this.initDraggableCards()
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
    }
}
