package acsm

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func DefaultCustomChecksums() *CustomChecksums {
	return &CustomChecksums{Entries: []CustomChecksumEntries{}}
}

type CustomChecksums struct {
	Entries []CustomChecksumEntries `json:"entries"`
}

type CustomChecksumEntries struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Filepath string    `json:"filepath"`
	Checksum string    `json:"checksum"`
}

type CustomChecksumHandler struct {
	*BaseHandler

	store Store
}

func NewCustomChecksumHandler(baseHandler *BaseHandler, store Store) *CustomChecksumHandler {
	return &CustomChecksumHandler{BaseHandler: baseHandler, store: store}
}

type customChecksumEditTemplateVars struct {
	BaseTemplateVars

	CustomChecksums *CustomChecksums
}

func (cch *CustomChecksumHandler) index(w http.ResponseWriter, r *http.Request) {
	data, err := cch.store.LoadCustomChecksums()

	if err != nil {
		logrus.Errorf("couldn't load required apps, err: %s", err)
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			logrus.Errorf("required apps: couldn't parse form, err: %s", err)
		}

		length := formValueAsInt(r.FormValue("Entries.NumEntries"))

		var entries []CustomChecksumEntries

		for i := 0; i < length; i++ {

			idString := r.Form["Entries.ID"][i]
			var id uuid.UUID

			if idString == "" {
				id = uuid.New()
			} else {
				id, err = uuid.Parse(idString)

				if err != nil {
					id = uuid.New()
				}
			}

			entries = append(entries, CustomChecksumEntries{
				ID:       id,
				Name:     r.Form["Entries.Name"][i],
				Filepath: r.Form["Entries.Filepath"][i],
				Checksum: r.Form["Entries.Checksum"][i],
			})
		}

		data = &CustomChecksums{
			Entries: entries,
		}

		// save the config
		err = cch.store.UpsertCustomChecksums(data)

		if err != nil {
			logrus.Errorf("Couldn't update required apps, err: %s", err)
			AddErrorFlash(w, r, "Failed to update required apps")
		} else {
			logrus.Debugf("Required apps successfully updated!")
			AddFlash(w, r, "Required apps successfully updated!")
		}
	}

	cch.viewRenderer.MustLoadTemplate(w, r, "server/custom-checksums.html", &customChecksumEditTemplateVars{
		CustomChecksums: data,
	})
}
