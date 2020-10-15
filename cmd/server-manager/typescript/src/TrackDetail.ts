import ClickEvent = JQuery.ClickEvent;

export class TrackDetail {
    public constructor() {
        if (!$(".track-details").length) {
            return;
        }

        $(".track-image").on("click", TrackDetail.onTrackLayoutClick);
        $(".recalculate-splines-button").on("click", TrackDetail.onRecalculateSplinesClick);

        TrackDetail.fixLayoutImageHeights();
        TrackDetail.initSummerNote();
        $(window).on("resize", TrackDetail.fixLayoutImageHeights);
        TrackDetail.showSplinesImageOnPopupLoad();
    }

    private static onTrackLayoutClick(e: ClickEvent) {
        const $currentTarget = $(e.currentTarget);

        $("#hero-skin").attr({
            "src": $currentTarget.attr("src"),
            "alt": $currentTarget.attr("alt"),
        });

        $("select[name='skin-delete']").val($currentTarget.data("layout"));
    }

    private static fixLayoutImageHeights() {
        $(".track-layouts").height($("#hero-skin").height()!);
    }

    private static initSummerNote() {
        let $summerNote = $("#summernote");
        let $trackNotes = $("#TrackNotes");

        if ($trackNotes.length > 0) {
            $summerNote.summernote('code', $trackNotes.html());
        }

        $summerNote.summernote({
            placeholder: 'You can use this text input to attach notes to each track!',
            tabsize: 2,
            height: 200,
        });
    }

    private static showSplinesImageOnPopupLoad() {
        let $pitlaneDetectionModal = $(".pitlane-detection-modal");

        $pitlaneDetectionModal.on("shown.bs.modal", () => {
            let $image = $(this).find("img.splines-image");

            // load in images when the modal is shown
            TrackDetail.lazyLoadImage($image.data("src"));
        });

        $pitlaneDetectionModal.on("hidden.bs.modal", () => {
            let $image = $(this).find("img.splines-image");
            $image.attr("src", "");
        });
    }

    private static lazyLoadImage(url: string) {
        let img = new Image();
        let $statusIndicator = $(".status-indicator");
        $statusIndicator.show();

        img.onload = () => {
            $(".splines-image").attr("src", url);
            $statusIndicator.hide();
        };

        img.src = url;
    }

    private static onRecalculateSplinesClick(e: ClickEvent) {
        e.preventDefault();

        const $currentTarget = $(e.currentTarget);
        let $form = $currentTarget.parent();
        let $modal = $form.parent();

        let baseURL: string = $modal.children(".splines-image-base-url").text() as string;

        let distance: number = $form.children(".distance").val() as number;
        let maxSpeed: number = $form.children(".maxSpeed").val() as number;
        let maxDistance: number = $form.children(".maxDistance").val() as number;

        let url = baseURL + "?distance=" + distance + "&maxSpeed=" + maxSpeed + "&maxDistance=" + maxDistance;

        TrackDetail.lazyLoadImage(url);
    }
}
