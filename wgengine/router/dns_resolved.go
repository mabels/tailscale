// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package router

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"
	"tailscale.com/net/interfaces"
)

// resolvedListenAddr is the listen address of the resolved stub resolver.
//
// We only consider resolved to be the system resolver if the stub resolver is;
// that is, if this address is the sole nameserver in /etc/resolved.conf.
// In other cases, resolved may still be managing the system DNS configuration directly.
// Then the nameserver list will be a concatenation of those for all
// the interfaces that register their interest in being a default resolver with
//   SetLinkDomains([]{{"~.", true}, ...})
// which includes at least the interface with the default route, i.e. not us.
// This does not work for us: there is a possibility of getting NXDOMAIN
// from the other nameservers before we are asked or get a chance to respond.
// We consider this case as lacking resolved support and fall through to dnsDirect.
//
// While it may seem that we need to read a config option to get at this,
// this address is, in fact, hard-coded into resolved.
var resolvedListenAddr = netaddr.IPv4(127, 0, 0, 53)

// dnsReconfigTimeout is the timeout for DNS reconfiguration.
//
// This is useful because certain conditions can cause indefinite hangs
// (such as improper dbus auth followed by contextless dbus.Object.Call).
// Such operations should be wrapped in a timeout context.
const dnsReconfigTimeout = time.Second

var errNotReady = errors.New("interface not ready")

type resolvedLinkNameserver struct {
	Family  int32
	Address []byte
}

type resolvedLinkDomain struct {
	Domain      string
	RoutingOnly bool
}

// resolvedIsActive determines if resolved is currently managing system DNS settings.
func resolvedIsActive() bool {
	// systemd-resolved is never installed without systemd.
	_, err := exec.LookPath("systemctl")
	if err != nil {
		return false
	}

	// is-active exits with code 3 if the service is not active.
	err = exec.Command("systemctl", "is-active", "systemd-resolved").Run()
	if err != nil {
		return false
	}

	config, err := dnsReadConfig()
	if err != nil {
		return false
	}

	// The sole nameserver must be the systemd-resolved stub.
	if len(config.Nameservers) == 1 && config.Nameservers[0] == resolvedListenAddr {
		return true
	}

	return false
}

// dnsResolvedUp sets the DNS parameters for the Tailscale interface
// to given nameservers and search domains using the resolved DBus API.
func dnsResolvedUp(config DNSConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), dnsReconfigTimeout)
	defer cancel()

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("connecting to system bus: %w", err)
	}
	defer conn.Close()

	resolved := conn.Object(
		"org.freedesktop.resolve1",
		dbus.ObjectPath("/org/freedesktop/resolve1"),
	)

	_, iface, err := interfaces.Tailscale()
	if err != nil {
		return fmt.Errorf("getting interface index: %w", err)
	}
	if iface == nil {
		return errNotReady
	}

	var linkNameservers = make([]resolvedLinkNameserver, len(config.Nameservers))
	for i, server := range config.Nameservers {
		ip := server.As16()
		if server.Is4() {
			linkNameservers[i] = resolvedLinkNameserver{
				Family:  unix.AF_INET,
				Address: ip[12:],
			}
		} else {
			linkNameservers[i] = resolvedLinkNameserver{
				Family:  unix.AF_INET6,
				Address: ip[:],
			}
		}
	}

	err = resolved.CallWithContext(
		ctx, "org.freedesktop.resolve1.Manager.SetLinkDNS", 0,
		iface.Index, linkNameservers,
	).Store()
	if err != nil {
		return fmt.Errorf("SetLinkDNS: %w", err)
	}

	var linkDomains = make([]resolvedLinkDomain, len(config.Domains))
	for i, domain := range config.Domains {
		linkDomains[i] = resolvedLinkDomain{
			Domain:      domain,
			RoutingOnly: false,
		}
	}

	err = resolved.CallWithContext(
		ctx, "org.freedesktop.resolve1.Manager.SetLinkDomains", 0,
		iface.Index, linkDomains,
	).Store()
	if err != nil {
		return fmt.Errorf("SetLinkDomains: %w", err)
	}

	return nil
}

// dnsResolvedDown undoes the changes made by dnsResolvedUp.
func dnsResolvedDown() error {
	ctx, cancel := context.WithTimeout(context.Background(), dnsReconfigTimeout)
	defer cancel()

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("connecting to system bus: %w", err)
	}

	resolved := conn.Object(
		"org.freedesktop.resolve1",
		dbus.ObjectPath("/org/freedesktop/resolve1"),
	)

	_, iface, err := interfaces.Tailscale()
	if err != nil {
		return fmt.Errorf("getting interface index: %w", err)
	}
	if iface == nil {
		return errNotReady
	}

	err = resolved.CallWithContext(
		ctx, "org.freedesktop.resolve1.Manager.RevertLink", 0,
		iface.Index,
	).Store()
	if err != nil {
		return fmt.Errorf("RevertLink: %w", err)
	}

	return nil
}
