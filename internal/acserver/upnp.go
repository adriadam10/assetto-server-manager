package acserver

import (
	"context"

	"gitlab.com/NebulousLabs/go-upnp"
)

type UPnP struct {
	httpPort, tcpPort, udpPort, contentManagerWrapperPort uint16

	router *upnp.IGD
}

func NewUPnP(httpPort, tcpPort, udpPort, contentManagerWrapperPort uint16) *UPnP {
	return &UPnP{
		httpPort:                  httpPort,
		tcpPort:                   tcpPort,
		udpPort:                   udpPort,
		contentManagerWrapperPort: contentManagerWrapperPort,
	}
}

func (u *UPnP) SetUp(ctx context.Context) error {
	var err error

	u.router, err = upnp.DiscoverCtx(ctx)

	if err != nil {
		return err
	}

	if err := u.router.Forward(u.httpPort, "acserver http"); err != nil {
		return err
	}

	if err := u.router.Forward(u.tcpPort, "acserver tcp"); err != nil {
		return err
	}

	if err := u.router.Forward(u.udpPort, "acserver udp"); err != nil {
		return err
	}

	if u.contentManagerWrapperPort > 0 {
		if err := u.router.Forward(u.contentManagerWrapperPort, "acserver content manager wrapper"); err != nil {
			return err
		}
	}

	return nil
}

func (u *UPnP) Teardown() error {
	if err := u.router.Clear(u.httpPort); err != nil {
		return err
	}

	if err := u.router.Clear(u.tcpPort); err != nil {
		return err
	}

	if err := u.router.Clear(u.udpPort); err != nil {
		return err
	}

	if u.contentManagerWrapperPort > 0 {
		if err := u.router.Clear(u.contentManagerWrapperPort); err != nil {
			return err
		}
	}

	return nil
}
