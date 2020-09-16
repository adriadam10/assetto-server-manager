export class CustomChecksums {
    private readonly $parent: JQuery<HTMLElement>;

    constructor() {
        this.$parent = $(".custom-checksum");

        if (this.$parent.length == 0) {
            return;
        }

        this.initialiseCustomChecksums();
    }

    private initialiseCustomChecksums(): void {
        let $tmplApp = this.$parent.find("#appTemplate");

        let $appTemplate = $tmplApp.prop("id", "").clone(true, true);
        $appTemplate.removeClass("d-none");

        $tmplApp.remove();

        let $savedNumApps = this.$parent.find(".Entries.NumEntries");
        $savedNumApps.val(this.$parent.find(".app-entry:visible").length);

        $(document).on("click", ".btn-delete-app", function (e) {
            deleteApp(e);
        });

        $(document).on("click", ".addEntries", function (e) {
            e.preventDefault();

            let $numEntriesField = $(this).parent().find(".numEntriesToAdd");
            let numEntriesToAdd = 1;

            if ($numEntriesField.length > 0) {
                numEntriesToAdd = $numEntriesField.val() as number;
            }

            let $clonedTemplate = $appTemplate.clone();
            let $currentCustomChecksum = $(e.target).closest(".custom-checksum");

            for (let i = 0; i < numEntriesToAdd; i++) {
                let $elem = $clonedTemplate.clone();

                $elem.appendTo($currentCustomChecksum.find(".app-block"));

                $elem.css("display", "block");
            }

            let $savedNumEntries = $currentCustomChecksum.find(".numEntries");
            $savedNumEntries.val($currentCustomChecksum.find(".app-entry:visible").length);
        });

        function deleteApp(e) {
            e.preventDefault();

            let $currentCustomChecksum = $(e.target).closest(".custom-checksum");

            $(e.target).closest(".app-entry").remove();

            let $savedNumEntries = $currentCustomChecksum.find(".numEntries");
            $savedNumEntries.val($currentCustomChecksum.find(".app-entry:visible").length);
        }
    }
}
