package updater

import (
	"errors"

	"github.com/go-co-op/gocron/v2"
	"github.com/ivpn/dns/blocklists/model"
)

const UpdaterTypeStandard = "standard"

type Updater interface {
	Setup(model.BlocklistMetadata, func() (*model.BlocklistMetadata, error)) error
	Start()
	Stop()
	Erase()
}

// New creates an Updater. locker may be nil to run without cross-instance
// coordination (tests, single-instance dev).
func New(updaterType string, locker gocron.Locker) (Updater, error) {
	switch updaterType { // nolint
	case UpdaterTypeStandard:
		return NewStandardUpdater(locker)
	}
	return nil, errors.New("unknown updater type")
}
